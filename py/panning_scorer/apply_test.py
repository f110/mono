"""CSV からの適用時の行→アクション決定 (再スコアリングなし, 設計 §8.4) を検証する。

resolve_action は純粋関数(XMP/libxmp に依存しない)なのでネイティブライブラリ無しで実行できる。
"""

import unittest

from apply import ACTION_NON_PANNING, ACTION_RATE, ACTION_SKIP, resolve_action
from config import Config
from rating_mapper import RatingMapper


class ResolveActionTest(unittest.TestCase):
    def setUp(self) -> None:
        self.cfg = Config()
        self.mapper = RatingMapper.from_config(self.cfg)
        self.factor = self.cfg.panning_shutter_factor

    def _act(self, score, focal, denom, filter_np=True):
        return resolve_action(score, focal, denom, self.mapper, self.factor, filter_np)

    def test_non_panning_excluded_even_with_score(self) -> None:
        # 600mm 1/200 は非流し撮り → score があっても対象外
        self.assertEqual(self._act(0.9, 600, 200)[0], ACTION_NON_PANNING)

    def test_filter_off_rates_everything(self) -> None:
        action, _ = self._act(1.0, 600, 200, filter_np=False)
        self.assertEqual(action, ACTION_RATE)

    def test_rate_recomputed_from_score(self) -> None:
        # 流し撮り(600mm 1/100)。score=1.0 → 最上位 Rating 4 に再マッピング
        action, rating = self._act(1.0, 600, 100)
        self.assertEqual(action, ACTION_RATE)
        self.assertEqual(rating, 4)

    def test_skip_when_no_score(self) -> None:
        # 流し撮りだが dry-run 時に score が無い → 書き込めない
        self.assertEqual(self._act(None, 600, 100)[0], ACTION_SKIP)

    def test_unknown_exif_fails_open(self) -> None:
        # EXIF 欠如 → 判定不能 → レーティングする
        action, _ = self._act(0.5, None, None)
        self.assertEqual(action, ACTION_RATE)


if __name__ == "__main__":
    unittest.main()
