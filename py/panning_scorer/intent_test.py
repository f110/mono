"""流し撮り意図判定 (焦点距離とシャッター速度のみ) の境界を検証する。

heavy な依存を持たない純粋ロジックのため、ネイティブライブラリ無しで実行できる。
"""

import unittest

from intent import ShotParams, is_panning_intent


class PanningIntentTest(unittest.TestCase):
    def test_examples_600mm(self) -> None:
        # 焦点距離600mm, factor=5 → 閾値=120。分母 < 120 なら流し撮り意図。
        self.assertTrue(is_panning_intent(ShotParams(600, 100), 5.0))   # 1/100 → 流し撮り
        self.assertTrue(is_panning_intent(ShotParams(600, 119), 5.0))
        self.assertFalse(is_panning_intent(ShotParams(600, 120), 5.0))  # 境界(=)は非流し撮り
        self.assertFalse(is_panning_intent(ShotParams(600, 200), 5.0))  # 1/200 → 非流し撮り

    def test_unknown_returns_none(self) -> None:
        self.assertIsNone(is_panning_intent(ShotParams(None, 100), 5.0))
        self.assertIsNone(is_panning_intent(ShotParams(600, None), 5.0))
        self.assertIsNone(is_panning_intent(ShotParams(0, 100), 5.0))
        self.assertIsNone(is_panning_intent(ShotParams(600, 0), 5.0))


if __name__ == "__main__":
    unittest.main()
