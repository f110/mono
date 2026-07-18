"""RatingMapper のスコア→Rating 境界 (設計 §6.2) を検証する。

heavy な依存を持たない純粋ロジックのため、ネイティブライブラリ無しで実行できる。
"""

import unittest

from config import Config
from rating_mapper import RatingMapper


class RatingMapperTest(unittest.TestCase):
    def setUp(self) -> None:
        self.mapper = RatingMapper([0.25, 0.45, 0.60, 0.75, 0.90], [-1, 0, 1, 2, 3, 4])

    def test_boundaries(self) -> None:
        # (score, 期待 Rating)。境界値そのものは上位区間 (設計 §6.2 の「0.25〜」は 0.25 を含む)。
        cases = [
            (0.0, -1),
            (0.24, -1),
            (0.25, 0),
            (0.44, 0),
            (0.45, 1),
            (0.59, 1),
            (0.60, 2),
            (0.74, 2),
            (0.75, 3),
            (0.89, 3),
            (0.90, 4),
            (1.0, 4),
        ]
        for score, want in cases:
            with self.subTest(score=score):
                self.assertEqual(self.mapper.to_rating(score), want)

    def test_values_length_validation(self) -> None:
        with self.assertRaises(ValueError):
            RatingMapper([0.5], [0])


class DefaultScoreMappingTest(unittest.TestCase):
    """既定 Config の score→Rating 写像 (2026-06-30 実データで暫定校正, §6.2) を検証する。

    score = min(center/center_norm + surround/surround_norm, 1.0) を Rating へ写像する。
    Rating 0 は使わない(0星/未評価は非流し撮りに予約)。境界は暫定(要 再レビュー校正)。
    """

    def setUp(self) -> None:
        self.mapper = RatingMapper.from_config(Config())

    def test_score_boundaries(self) -> None:
        cases = [
            (0.0, -1), (0.371, -1),
            (0.3725, 1), (0.52, 1),
            (0.5217, 2), (0.72, 2),
            (0.7242, 3), (0.999, 3),
            (1.0, 4),
        ]
        for score, want in cases:
            with self.subTest(score=score):
                self.assertEqual(self.mapper.to_rating(score), want)


if __name__ == "__main__":
    unittest.main()
