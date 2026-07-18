"""NEF の埋め込みプレビュー、または抽出済み画像ファイルをグレースケールで返す (設計 §3.2, §5.1)。

NEF はフル RAW デコード(demosaic)を避け、LibRaw(rawpy) の埋め込みプレビューを使う。
JPEG/PNG 等の画像ファイル(サンプルデータの縮小プレビュー等)は拡張子で判別しそのままデコードする。
"""

from __future__ import annotations

import cv2
import numpy as np
import rawpy

# rawpy(埋め込みプレビュー抽出)で扱う拡張子。それ以外は画像ファイルとして直接デコードする。
_RAW_SUFFIXES = (".nef",)


class PreviewError(Exception):
    """プレビューを取得・デコードできない (設計 §5.2, §9)。スキップ対象。"""


class PreviewExtractor:
    def extract_gray(self, path: str) -> np.ndarray:
        gray = self._from_raw(path) if path.lower().endswith(_RAW_SUFFIXES) else self._from_image(path)
        if gray is None or gray.size == 0:
            raise PreviewError("failed to decode preview")
        return gray

    def _from_raw(self, path: str) -> np.ndarray:
        try:
            with rawpy.imread(path) as raw:
                thumb = raw.extract_thumb()
                return self._to_gray(thumb)
        except rawpy.LibRawNoThumbnailError as e:
            raise PreviewError("no embedded preview") from e
        except rawpy.LibRawUnsupportedThumbnailError as e:
            raise PreviewError("unsupported embedded preview") from e
        except rawpy.LibRawError as e:
            raise PreviewError(f"libraw error: {e}") from e

    @staticmethod
    def _from_image(path: str) -> np.ndarray:
        gray = cv2.imread(path, cv2.IMREAD_GRAYSCALE)
        if gray is None:
            raise PreviewError("failed to decode image")
        return gray

    @staticmethod
    def _to_gray(thumb) -> np.ndarray:
        if thumb.format == rawpy.ThumbFormat.JPEG:
            buf = np.frombuffer(thumb.data, dtype=np.uint8)
            return cv2.imdecode(buf, cv2.IMREAD_GRAYSCALE)
        # BITMAP: RGB の ndarray。with ブロック内で処理するためコピー不要。
        data = np.asarray(thumb.data)
        if data.ndim == 3:
            return cv2.cvtColor(data, cv2.COLOR_RGB2GRAY)
        return data
