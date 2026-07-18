"""NEF から焦点距離・シャッター速度を読む (流し撮り意図判定用, §4.5)。

exifread(純Python)で標準 EXIF タグを読む。フル RAW デコードは不要で、
ヘッダ部の読み取りのみのため高速。取得できないタグは None にする。
"""

from __future__ import annotations

import logging
from typing import Optional, Tuple

import exifread

from intent import ShotParams

logger = logging.getLogger("panning_scorer")


class ExifReader:
    def read(self, path: str) -> ShotParams:
        try:
            with open(path, "rb") as f:
                tags = exifread.process_file(f, details=False)
        except Exception as e:  # 読み取り失敗は判定不能扱い (全体は止めない)
            logger.debug("exif read failed for %s: %s", path, e)
            return ShotParams()
        return ShotParams(
            focal_length_mm=_focal_length(tags.get("EXIF FocalLength")),
            shutter_denominator=_shutter_denominator(tags.get("EXIF ExposureTime")),
        )


def _rational(tag) -> Optional[Tuple[float, float]]:
    if tag is None or not getattr(tag, "values", None):
        return None
    v = tag.values[0]
    num = getattr(v, "numerator", None)
    den = getattr(v, "denominator", None)
    if num is None:
        num = getattr(v, "num", None)
    if den is None:
        den = getattr(v, "den", None)
    if num is not None and den is not None:
        return float(num), float(den)
    try:
        return float(v), 1.0
    except (TypeError, ValueError):
        return None


def _focal_length(tag) -> Optional[float]:
    r = _rational(tag)
    if r is None or r[1] == 0:
        return None
    return r[0] / r[1]


def _shutter_denominator(tag) -> Optional[float]:
    # ExposureTime = num/den 秒。1/D 表記の分母 D = den/num = 1/秒。
    r = _rational(tag)
    if r is None or r[0] <= 0:
        return None
    return r[1] / r[0]
