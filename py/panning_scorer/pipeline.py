"""1 ファイルの EXIF判定→抽出→スコア→変換→XMP書き込み を独立して完結させる (設計 §7.2)。

各ワーカープロセスがこのパイプラインを 1 つ保持し、I/O をプロセス内に閉じる。
どの段でも例外で全体を止めず、Result として結果/理由を返す (設計 NFR-3, §9)。
"""

from __future__ import annotations

from config import Config
from exif import ExifReader
from intent import is_panning_intent
from preview_extractor import PreviewError, PreviewExtractor
from rating_mapper import RatingMapper
from result import (
    STATUS_FAILED,
    STATUS_NON_PANNING,
    STATUS_SCORED,
    STATUS_SKIPPED,
    STATUS_WRITTEN,
    Result,
)
from scorer import Scorer
from xmp_writer import XmpWriteError, XmpWriter


class Pipeline:
    def __init__(self, config: Config, write: bool, filter_non_panning: bool) -> None:
        self._write = write
        self._filter_non_panning = filter_non_panning
        self._factor = config.panning_shutter_factor
        self._exif = ExifReader()
        self._extractor = PreviewExtractor()
        self._scorer = Scorer(config)
        self._mapper = RatingMapper.from_config(config)
        self._writer = XmpWriter()

    def process(self, path: str) -> Result:
        # まず EXIF から流し撮り意図を判定する。非流し撮りはレーティング対象外 (設計 §4.5)。
        params = self._exif.read(path)
        exif = dict(focal_length=params.focal_length_mm, shutter_denom=params.shutter_denominator)
        if self._filter_non_panning and is_panning_intent(params, self._factor) is False:
            return Result(path=path, status=STATUS_NON_PANNING, reason="non-panning", **exif)
        # 意図不明(EXIF欠如)は評価を続行する (fail-open)。

        try:
            gray = self._extractor.extract_gray(path)
        except PreviewError as e:
            return Result(path=path, status=STATUS_SKIPPED, reason=str(e), **exif)
        except Exception as e:  # 想定外のデコード失敗
            return Result(path=path, status=STATUS_FAILED, reason=f"preview: {e}", **exif)

        try:
            s = self._scorer.score(gray)
        except Exception as e:
            return Result(path=path, status=STATUS_FAILED, reason=f"score: {e}", **exif)

        rating = self._mapper.to_rating(s.score)
        result = Result(
            path=path,
            status=STATUS_SCORED,
            score=s.score,
            rating=rating,
            center_sharpness=s.center_sharpness,
            surround_sharpness=s.surround_sharpness,
            ratio=s.ratio,
            **exif,
        )
        if not self._write:
            return result

        try:
            self._writer.write_rating(path, rating)
        except XmpWriteError as e:  # 書き込み失敗 → 当該のみ失敗 (設計 §9)
            result.status = STATUS_FAILED
            result.reason = f"xmp: {e}"
        else:
            result.status = STATUS_WRITTEN
        return result
