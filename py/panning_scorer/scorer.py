"""グリッド分割 Laplacian 分散で被写体シャープネスを評価する (設計 §3.2 Scorer, §4)。

実データ校正では、人間の採否は 中央/周辺比(AUC 0.40, 無力)より、中央と周辺の絶対
シャープネスの重み付き和(center+surround, AUC 0.83)でよく説明できた。よって
  score = min(center/center_norm + surround/surround_norm, 1.0)
をスコアとする。比(ratio)は診断用に CSV へ残す。
"""

from __future__ import annotations

from dataclasses import dataclass

import cv2
import numpy as np

from config import Config


@dataclass
class ScoreResult:
    score: float               # 正規化後スコア(0.0〜1.0) = min(center/center_norm + surround/surround_norm, 1)
    center_sharpness: float    # 中央領域の平均 Laplacian 分散
    surround_sharpness: float  # 周辺領域の平均 Laplacian 分散
    ratio: float               # 中央/周辺 の比 (診断用)


class Scorer:
    _EPS = 1e-6

    def __init__(self, config: Config) -> None:
        self._long_edge = config.downsample_long_edge
        self._n = config.grid_n
        self._center_norm = config.center_norm
        self._surround_norm = config.surround_norm

    def score(self, gray: np.ndarray) -> ScoreResult:
        gray = self._downsample(gray)
        cell_var = self._grid_laplacian_var(gray)
        center_mask = self._center_mask(self._n)
        center = float(cell_var[center_mask].mean())
        surround = float(cell_var[~center_mask].mean())
        ratio = center / max(surround, self._EPS)
        score = min(center / self._center_norm + surround / self._surround_norm, 1.0)
        return ScoreResult(score, center, surround, ratio)

    def _downsample(self, gray: np.ndarray) -> np.ndarray:
        h, w = gray.shape[:2]
        long_edge = max(h, w)
        if long_edge <= self._long_edge:
            return gray
        scale = self._long_edge / long_edge
        size = (max(1, round(w * scale)), max(1, round(h * scale)))
        return cv2.resize(gray, size, interpolation=cv2.INTER_AREA)

    def _grid_laplacian_var(self, gray: np.ndarray) -> np.ndarray:
        # Laplacian は全体で 1 回だけ計算し、各セルは分散のみを取る。
        lap = cv2.Laplacian(gray, cv2.CV_64F)
        h, w = lap.shape
        n = self._n
        rows = np.linspace(0, h, n + 1, dtype=int)
        cols = np.linspace(0, w, n + 1, dtype=int)
        out = np.empty((n, n), dtype=np.float64)
        for i in range(n):
            for j in range(n):
                cell = lap[rows[i]:rows[i + 1], cols[j]:cols[j + 1]]
                out[i, j] = float(cell.var()) if cell.size else 0.0
        return out

    @staticmethod
    def _center_mask(n: int) -> np.ndarray:
        # 中央 2×2 領域 (設計 §4.2)。N=4 で中央 4 セル。
        mask = np.zeros((n, n), dtype=bool)
        lo = (n - 2) // 2
        mask[lo:lo + 2, lo:lo + 2] = True
        return mask
