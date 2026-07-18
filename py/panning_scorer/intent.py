"""流し撮り意図の判定 (焦点距離とシャッター速度のみで決定)。

heavy な依存を持たない純粋モジュール。
シャッター速度の分母(1/D の D)が 焦点距離/factor より小さいとき流し撮り意図とみなす。
例: 600mm では 1/100(分母100 < 600/5=120)は流し撮り、1/200(分母200)は非流し撮り。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional


@dataclass
class ShotParams:
    focal_length_mm: Optional[float] = None
    shutter_denominator: Optional[float] = None  # 1/D 表記の D。1/200 なら 200。


def is_panning_intent(params: ShotParams, factor: float) -> Optional[bool]:
    """流し撮り意図なら True、非流し撮りなら False、判定不能(EXIF欠如)なら None を返す。"""
    focal = params.focal_length_mm
    denom = params.shutter_denominator
    if focal is None or denom is None or focal <= 0 or denom <= 0:
        return None
    return denom < focal / factor
