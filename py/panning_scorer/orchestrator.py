"""ファイル走査・並列実行・進捗管理・結果集約を行う (設計 §3.2 Orchestrator, §7)。

CPU バウンドかつネイティブライブラリ主体のため GIL を回避してプロセス並列を採用する。
macOS では fork がネイティブライブラリと相性が悪いため spawn を用いる。
"""

from __future__ import annotations

import csv
import logging
import multiprocessing as mp
import os
import subprocess
from typing import List, Optional

from config import Config
from pipeline import Pipeline
from result import (
    CSV_FIELDS,
    STATUS_FAILED,
    STATUS_NON_PANNING,
    STATUS_SCORED,
    STATUS_SKIPPED,
    STATUS_WRITTEN,
    Result,
    Summary,
)

logger = logging.getLogger("panning_scorer")

# NEF に加え、抽出済みプレビュー画像(サンプルデータ等)も入力に取れる。抽出は拡張子で分岐 (preview_extractor)。
_INPUT_SUFFIXES = (".nef", ".jpg", ".jpeg", ".png", ".tif", ".tiff")

# spawn されたワーカーで共有するパイプライン (プロセスごとに 1 つ)。
_pipeline: Optional[Pipeline] = None


def _init_worker(config: Config, write: bool, filter_non_panning: bool) -> None:
    global _pipeline
    _pipeline = Pipeline(config, write=write, filter_non_panning=filter_non_panning)


def _run_one(path: str) -> Result:
    assert _pipeline is not None
    return _pipeline.process(path)


def find_input_files(root: str) -> List[str]:
    out: List[str] = []
    for dirpath, _dirs, files in os.walk(root):
        for name in files:
            if name.lower().endswith(_INPUT_SUFFIXES):
                out.append(os.path.join(dirpath, name))
    out.sort()
    return out


def default_workers() -> int:
    # Apple Silicon の性能(P)コア数を優先する (設計 §7.2)。
    try:
        proc = subprocess.run(
            ["sysctl", "-n", "hw.perflevel0.physicalcpu"],
            capture_output=True, text=True, check=True,
        )
        v = int(proc.stdout.strip())
        if v > 0:
            return v
    except Exception:
        pass
    return os.cpu_count() or 1


class Orchestrator:
    def __init__(self, config: Config, write: bool, filter_non_panning: bool = True,
                 workers: Optional[int] = None) -> None:
        self._config = config
        self._write = write
        self._filter_non_panning = filter_non_panning
        self._workers = workers if workers and workers > 0 else default_workers()

    def run(self, root: str, csv_path: Optional[str] = None) -> Summary:
        files = find_input_files(root)
        total = len(files)
        logger.info("found %d input files (workers=%d, write=%s)", total, self._workers, self._write)

        results: List[Result] = []
        for i, res in enumerate(self._iter_results(files), 1):
            results.append(res)
            if res.status in (STATUS_SKIPPED, STATUS_FAILED):
                logger.warning("%s: %s (%s)", res.path, res.status, res.reason)
            if i % 200 == 0 or i == total:
                logger.info("progress %d/%d", i, total)

        if csv_path:
            self._write_csv(csv_path, results)
            logger.info("wrote score distribution CSV: %s", csv_path)
        return self._summarize(results)

    def _iter_results(self, files: List[str]):
        if self._workers == 1:
            # デバッグ・プロファイル用の直列実行。
            _init_worker(self._config, self._write, self._filter_non_panning)
            for path in files:
                yield _run_one(path)
            return
        ctx = mp.get_context("spawn")
        with ctx.Pool(
            processes=self._workers,
            initializer=_init_worker,
            initargs=(self._config, self._write, self._filter_non_panning),
        ) as pool:
            yield from pool.imap_unordered(_run_one, files, chunksize=8)

    @staticmethod
    def _write_csv(path: str, results: List[Result]) -> None:
        with open(path, "w", newline="", encoding="utf-8") as f:
            w = csv.writer(f)
            w.writerow(CSV_FIELDS)
            for r in results:
                w.writerow(r.as_csv_row())

    @staticmethod
    def _summarize(results: List[Result]) -> Summary:
        s = Summary()
        s.total = len(results)
        for r in results:
            if r.status == STATUS_WRITTEN:
                s.written += 1
            elif r.status == STATUS_SCORED:
                s.scored += 1
            elif r.status == STATUS_SKIPPED:
                s.skipped += 1
            elif r.status == STATUS_NON_PANNING:
                s.non_panning += 1
            elif r.status == STATUS_FAILED:
                s.failed += 1
            if r.rating is not None:
                s.rating_dist[r.rating] += 1
        return s
