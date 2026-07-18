"""NEF の埋め込みプレビューを縮小 JPEG として書き出し、開発用サンプルデータを作る。

NEF 本体(1枚~30MB)は大きすぎるため、判定に使う埋め込みプレビューだけを抽出し、
長辺を一定サイズへ縮小して保存する。ディレクトリ構造(例: `_Rejected/`)を保つので、
人手選別ラベルはそのまま残る。特徴量計算アルゴリズムを変える実験に使える。

    bazel run //py/panning_scorer:export-previews -- <input_dir> <output_dir> [--max-edge 2048]
"""

from __future__ import annotations

import argparse
import logging
import multiprocessing as mp
import os
import sys
from typing import List, Optional, Tuple

import cv2
import numpy as np
import rawpy

logger = logging.getLogger("export_previews")

_NEF_SUFFIXES = (".nef",)


def _find_nef(root: str) -> List[str]:
    out: List[str] = []
    for dirpath, _dirs, files in os.walk(root):
        for name in files:
            if name.lower().endswith(_NEF_SUFFIXES):
                out.append(os.path.join(dirpath, name))
    out.sort()
    return out


def _dst_path(src: str, input_dir: str, output_dir: str) -> str:
    rel = os.path.relpath(src, input_dir)
    return os.path.join(output_dir, os.path.splitext(rel)[0] + ".jpg")


# spawn ワーカーで共有する設定
_CFG: Optional[Tuple[str, str, int, int, bool]] = None


def _init(input_dir: str, output_dir: str, max_edge: int, quality: int, force: bool) -> None:
    global _CFG
    _CFG = (input_dir, output_dir, max_edge, quality, force)


def _export_one(src: str) -> Tuple[str, Optional[str]]:
    assert _CFG is not None
    input_dir, output_dir, max_edge, quality, force = _CFG
    dst = _dst_path(src, input_dir, output_dir)
    if not force and os.path.exists(dst):
        return src, None  # 再開: 既存はスキップ
    try:
        with rawpy.imread(src) as raw:
            thumb = raw.extract_thumb()
            if thumb.format == rawpy.ThumbFormat.JPEG:
                img = cv2.imdecode(np.frombuffer(thumb.data, np.uint8), cv2.IMREAD_COLOR)
            else:  # BITMAP (RGB)
                img = cv2.cvtColor(np.asarray(thumb.data), cv2.COLOR_RGB2BGR)
    except Exception as e:  # プレビュー無し等はスキップして続行
        return src, f"extract failed: {e}"
    if img is None or img.size == 0:
        return src, "decode failed"
    h, w = img.shape[:2]
    long_edge = max(h, w)
    if long_edge > max_edge:
        scale = max_edge / long_edge
        img = cv2.resize(img, (max(1, round(w * scale)), max(1, round(h * scale))), interpolation=cv2.INTER_AREA)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    if not cv2.imwrite(dst, img, [cv2.IMWRITE_JPEG_QUALITY, quality]):
        return src, "write failed"
    return src, None


def main(argv: Optional[List[str]] = None) -> int:
    p = argparse.ArgumentParser(prog="export-previews", description=__doc__)
    p.add_argument("input_dir", help="NEF が入ったディレクトリ")
    p.add_argument("output_dir", help="縮小プレビュー(.jpg)の出力先。相対構造を保つ")
    p.add_argument("--max-edge", type=int, default=2048, help="長辺の上限px (既定2048)")
    p.add_argument("--quality", type=int, default=92, help="JPEG 品質 (既定92)")
    p.add_argument("--workers", type=int, default=os.cpu_count() or 1, help="並列ワーカー数")
    p.add_argument("--force", action="store_true", help="既存の出力も上書きする")
    args = p.parse_args(argv if argv is not None else sys.argv[1:])
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    files = _find_nef(args.input_dir)
    total = len(files)
    logger.info("found %d NEF, exporting previews to %s (max_edge=%d, workers=%d)",
                total, args.output_dir, args.max_edge, args.workers)

    ok = failed = 0
    ctx = mp.get_context("spawn")
    initargs = (args.input_dir, args.output_dir, args.max_edge, args.quality, args.force)
    with ctx.Pool(processes=args.workers, initializer=_init, initargs=initargs) as pool:
        for i, (src, err) in enumerate(pool.imap_unordered(_export_one, files, chunksize=8), 1):
            if err:
                failed += 1
                logger.warning("%s: %s", src, err)
            else:
                ok += 1
            if i % 500 == 0 or i == total:
                logger.info("progress %d/%d (ok=%d failed=%d)", i, total, ok, failed)
    logger.info("done: ok=%d failed=%d", ok, failed)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
