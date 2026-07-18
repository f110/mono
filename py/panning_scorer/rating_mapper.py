"""スコア(0.0〜1.0) を XMP Rating(-1〜5) へ変換する (設計 §3.2 Rating Mapper, §6.2)。

閾値による純粋な写像のみ。heavy な依存を持たないため単体テストが容易。
"""

from __future__ import annotations

import bisect
from typing import List


class RatingMapper:
    def __init__(self, thresholds: List[float], values: List[int]) -> None:
        if len(values) != len(thresholds) + 1:
            raise ValueError("values must have exactly one more element than thresholds")
        self._thresholds = list(thresholds)
        self._values = list(values)

    @classmethod
    def from_config(cls, config) -> "RatingMapper":
        return cls(config.rating_thresholds, config.rating_values)

    def to_rating(self, score: float) -> int:
        # bisect_right により境界値そのものは上位区間に含める (例: 0.25 → Rating 0)。
        return self._values[bisect.bisect_right(self._thresholds, score)]
