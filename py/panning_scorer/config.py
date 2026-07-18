"""パイプライン全体の調整対象パラメータ (設計 §10)。

heavy な依存を持たない純粋モジュール。ワーカープロセスへ pickle して渡す。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List

# スコアは中央+周辺の絶対シャープネスの重み付き和で算出する (設計 §4)。
#   score = min(center_sharpness / center_norm + surround_sharpness / surround_norm, 1.0)
# 2026-06-30 の実データ(レビュー済み rating>=1 プール, 代表セッション)で、keep/reject の分離は
# ratio(AUC 0.40, 無力) < center 単体(0.71) < surround(0.76) < center+surround(0.83) だった。
# 重みは各特徴の標準偏差に反比例(z-sum相当): center_norm:surround_norm ≒ std(center):std(surround)。
# 区切り: -1 | 1 | 2 | 3 | 4。Rating 0 は使わない(0星/未評価は非流し撮りに予約)。


@dataclass(frozen=True)
class Config:
    # 判定用画像サイズ。長辺をこのサイズへダウンサンプルする。速度と精度のトレードオフ (設計 §4.2)。
    downsample_long_edge: int = 1600
    # N×N のグリッド分割数。構図依存性に影響する (設計 §4.2)。
    grid_n: int = 4
    # スコア合成の正規化係数 (設計 §4)。std(center)/std(surround)≒3.3 の重み付けを畳み込んだ値。
    center_norm: float = 737.0
    surround_norm: float = 224.0
    # 流し撮り意図の判定係数 (設計 §4.5)。シャッター分母 < 焦点距離/factor なら流し撮り意図。
    panning_shutter_factor: float = 5.0
    # score→Rating 変換の境界値 (設計 §6.2)。score 空間(0.0〜1.0)。※暫定(要 再レビュー校正)。
    rating_thresholds: List[float] = field(default_factory=lambda: [0.3725, 0.5217, 0.7242, 1.0])
    # 各区間に対応する Rating。len == len(rating_thresholds) + 1 (設計 §6.2)。Rating 0 は使わない。
    rating_values: List[int] = field(default_factory=lambda: [-1, 1, 2, 3, 4])

    def __post_init__(self) -> None:
        if self.downsample_long_edge < 1:
            raise ValueError("downsample_long_edge must be >= 1")
        # 中央 2×2 領域と周辺領域の両方が非空になる最小値が 3。
        if self.grid_n < 3:
            raise ValueError("grid_n must be >= 3")
        if self.center_norm <= 0 or self.surround_norm <= 0:
            raise ValueError("center_norm and surround_norm must be > 0")
        if self.panning_shutter_factor <= 0:
            raise ValueError("panning_shutter_factor must be > 0")
        if len(self.rating_values) != len(self.rating_thresholds) + 1:
            raise ValueError("rating_values must have exactly one more element than rating_thresholds")
        if any(self.rating_thresholds[i] >= self.rating_thresholds[i + 1] for i in range(len(self.rating_thresholds) - 1)):
            raise ValueError("rating_thresholds must be strictly increasing")
