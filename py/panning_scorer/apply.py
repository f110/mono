"""dry-run で生成した CSV から XMP サイドカーを更新する (再スコアリングしない, 設計 §8.4)。

CSV は高コストな計算(score, EXIF)のキャッシュとして扱い、安価な Rating 写像と流し撮り
判定のみを現在の設定で再適用する。閾値・判定係数を dry-run 後に調整しても、再スコアリング
不要で XMP へ反映できる。スコアリング系(cv2/rawpy 等)には依存しない。
"""

from __future__ import annotations

import csv
import logging
from typing import Optional, Tuple

from config import Config
from intent import ShotParams, is_panning_intent
from rating_mapper import RatingMapper
from result import Summary

logger = logging.getLogger("panning_scorer")

# resolve_action の返り値 action
ACTION_RATE = "rate"                # rating を書き込む
ACTION_SKIP = "skip"               # score が無くレーティングできない
ACTION_NON_PANNING = "non_panning"  # 非流し撮りとして対象外


def resolve_action(
    score: Optional[float],
    focal_length: Optional[float],
    shutter_denom: Optional[float],
    mapper: RatingMapper,
    panning_factor: float,
    filter_non_panning: bool,
) -> Tuple[str, Optional[int]]:
    """CSV 1 行から (action, rating) を決める純粋関数。

    流し撮り判定と Rating 写像のみで、スコアや EXIF の再計算はしない。
    """
    if filter_non_panning and is_panning_intent(ShotParams(focal_length, shutter_denom), panning_factor) is False:
        return ACTION_NON_PANNING, None
    if score is None:
        # dry-run 時にスコアが無い(スキップ/非流し撮り/失敗)。書き込めない。
        return ACTION_SKIP, None
    return ACTION_RATE, mapper.to_rating(score)


def apply_from_csv(csv_path: str, config: Config, filter_non_panning: bool = True, write: bool = True) -> Summary:
    mapper = RatingMapper.from_config(config)
    summary = Summary()
    writer = None
    if write:
        # xmp_writer(libxmp/Exempi) は書き込み時のみ必要。dry-run(再マッピング確認)や
        # resolve_action の単体テストが Exempi 無しで動くよう遅延 import する。
        from xmp_writer import XmpWriteError, XmpWriter
        writer = XmpWriter()
    with open(csv_path, newline="", encoding="utf-8") as f:
        for row in csv.DictReader(f):
            summary.total += 1
            action, rating = resolve_action(
                _parse_float(row.get("score")),
                _parse_float(row.get("focal_length")),
                _parse_float(row.get("shutter_denom")),
                mapper,
                config.panning_shutter_factor,
                filter_non_panning,
            )
            if action == ACTION_NON_PANNING:
                summary.non_panning += 1
                continue
            if action == ACTION_SKIP:
                summary.skipped += 1
                continue
            summary.rating_dist[rating] += 1
            if not write:  # dry-run: 再マッピング結果の分布だけ確認する
                summary.scored += 1
                continue
            try:
                writer.write_rating(row.get("path", ""), rating)
            except XmpWriteError as e:
                logger.warning("%s: xmp write failed (%s)", row.get("path", ""), e)
                summary.failed += 1
            else:
                summary.written += 1
    return summary


def _parse_float(s: Optional[str]) -> Optional[float]:
    s = (s or "").strip()
    if not s:
        return None
    try:
        return float(s)
    except ValueError:
        return None
