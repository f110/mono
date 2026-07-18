"""1 ファイルの処理結果 (設計 §6 CSV / §9 サマリ用)。

pickle 可能なプリミティブのみで構成し、ワーカーからメインへ返す。
"""

from __future__ import annotations

from collections import Counter
from dataclasses import dataclass, field
from typing import List, Optional

# status の値
STATUS_SCORED = "scored"           # スコア算出成功 (dry-run 時の正常終了)
STATUS_WRITTEN = "written"         # XMP への書き込みまで成功
STATUS_SKIPPED = "skipped"         # プレビュー無し等でスキップ (設計 §9)
STATUS_NON_PANNING = "non_panning" # 流し撮り意図なしと判定しレーティング対象外 (設計 §4.5)
STATUS_FAILED = "failed"           # 予期しない失敗。全体処理は継続する (設計 NFR-3)

CSV_FIELDS: List[str] = [
    "path",
    "status",
    "score",
    "rating",
    "center_sharpness",
    "surround_sharpness",
    "ratio",
    "focal_length",
    "shutter_denom",
    "reason",
]


@dataclass
class Result:
    path: str
    status: str
    score: Optional[float] = None
    rating: Optional[int] = None
    center_sharpness: Optional[float] = None
    surround_sharpness: Optional[float] = None
    ratio: Optional[float] = None
    focal_length: Optional[float] = None    # EXIF 焦点距離(mm)
    shutter_denom: Optional[float] = None    # シャッター速度の分母 (1/D の D)
    reason: str = ""  # skip / fail / non-panning の理由

    def as_csv_row(self) -> List[object]:
        return [
            self.path,
            self.status,
            _fmt(self.score),
            "" if self.rating is None else self.rating,
            _fmt(self.center_sharpness),
            _fmt(self.surround_sharpness),
            _fmt(self.ratio),
            _fmt(self.focal_length),
            _fmt(self.shutter_denom),
            self.reason,
        ]


def _fmt(v: Optional[float]) -> str:
    return "" if v is None else f"{v:.6f}"


@dataclass
class Summary:
    total: int = 0
    scored: int = 0       # dry-run で正常にスコア算出できた件数
    written: int = 0      # XMP へ書き込めた件数
    skipped: int = 0
    non_panning: int = 0  # 非流し撮りとしてレーティング対象外 (設計 §4.5)
    failed: int = 0
    rating_dist: Counter = field(default_factory=Counter)
