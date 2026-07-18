"""流し撮り写真(Nikon NEF)自動レーティングツールの CLI エントリポイント (設計 §8)。

被写体シャープネスをスコアリングし XMP Rating へ書き込む一次スクリーニング。
最終判定は人間が行う。全自動での削除・除外は行わない (設計 §1)。
"""

from __future__ import annotations

import argparse
import logging
import sys
from typing import List, Optional

from config import Config
from orchestrator import Orchestrator, default_workers
from result import Summary


def _parse_args(argv: List[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="panning-scorer",
        description="流し撮り写真(Nikon NEF)を被写体シャープネスでスコアリングし XMP Rating に書き込む",
    )
    p.add_argument("input_dir", nargs="?", default=None, help="NEF ファイルが入ったディレクトリ (通常モード)")
    p.add_argument("--from-csv", metavar="PATH",
                   help="dry-run で出力した CSV から XMP を更新する。再スコアリングしない (設計 §8.4)")
    p.add_argument("--dry-run", action="store_true", help="XMP へ書き込まずスコアのみ算出する (設計 §8.1)")
    p.add_argument("--csv", metavar="PATH", help="スコア分布 CSV の出力先。閾値調整に使う (設計 §6, §8.1)")
    p.add_argument("--workers", type=int, default=None, help=f"並列ワーカー数 (既定: 性能コア数={default_workers()})")
    p.add_argument("--no-panning-filter", action="store_true",
                   help="流し撮り意図の判定を無効化し全ファイルをレーティングする (設計 §4.5)")

    g = p.add_argument_group("scoring parameters (設計 §10)")
    g.add_argument("--downsample", type=int, default=1600, help="ダウンサンプル長辺(px)")
    g.add_argument("--grid-n", type=int, default=4, help="グリッド分割数 N (N×N, 3 以上)")
    g.add_argument("--center-norm", type=float, default=Config().center_norm,
                   help="スコア合成 score=min(center/center_norm + surround/surround_norm, 1) の中央側正規化")
    g.add_argument("--surround-norm", type=float, default=Config().surround_norm,
                   help="同スコア合成の周辺側正規化。center_norm と併せて再校正に使う")
    g.add_argument("--panning-shutter-factor", type=float, default=Config().panning_shutter_factor,
                   help="流し撮り判定係数: シャッター分母 < 焦点距離/この値 なら流し撮り意図 (設計 §4.5)")

    p.add_argument("-v", "--verbose", action="store_true", help="デバッグログを出力する")
    return p.parse_args(argv)


def _build_config(args: argparse.Namespace) -> Config:
    return Config(
        downsample_long_edge=args.downsample,
        grid_n=args.grid_n,
        center_norm=args.center_norm,
        surround_norm=args.surround_norm,
        panning_shutter_factor=args.panning_shutter_factor,
    )


def _print_summary(summary: Summary) -> None:
    logger = logging.getLogger("panning_scorer")
    logger.info("---- summary ----")
    logger.info(
        "total=%d scored=%d written=%d skipped=%d non_panning=%d failed=%d",
        summary.total, summary.scored, summary.written, summary.skipped, summary.non_panning, summary.failed,
    )
    dist = ", ".join(f"{r}:{summary.rating_dist[r]}" for r in sorted(summary.rating_dist))
    logger.info("rating distribution: %s", dist or "(none)")


def main(argv: Optional[List[str]] = None) -> int:
    args = _parse_args(argv if argv is not None else sys.argv[1:])
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
    )
    logger = logging.getLogger("panning_scorer")
    if not args.input_dir and not args.from_csv:
        logger.error("input_dir か --from-csv のどちらかを指定してください")
        return 2
    if args.input_dir and args.from_csv:
        logger.error("input_dir と --from-csv は同時に指定できません")
        return 2

    config = _build_config(args)
    filter_non_panning = not args.no_panning_filter

    if args.from_csv:
        # 再スコアリングせず、CSV のキャッシュから Rating 写像・流し撮り判定のみ再適用する (設計 §8.4)。
        from apply import apply_from_csv
        summary = apply_from_csv(args.from_csv, config, filter_non_panning=filter_non_panning, write=not args.dry_run)
    else:
        if args.dry_run and not args.csv:
            logger.warning("dry-run without --csv: 結果はサマリのみ出力されます")
        orch = Orchestrator(
            config,
            write=not args.dry_run,
            filter_non_panning=filter_non_panning,
            workers=args.workers,
        )
        summary = orch.run(args.input_dir, csv_path=args.csv)
    _print_summary(summary)
    # 失敗ファイルがあっても全体処理は完了させる (設計 NFR-3)。終了コードで失敗有無を通知。
    return 1 if summary.failed else 0


if __name__ == "__main__":
    sys.exit(main())
