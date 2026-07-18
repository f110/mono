"""XMP サイドカーへ Rating を安全に書き込む (設計 §3.2 XMP Writer, §6)。

既存 XMP には FastRAWViewer 等が書いた他メタデータが含まれうるため、
ファイル全体を再生成せず、既存を更新モードで開いて Rating のみ上書きする (設計 §6.3)。
Exempi(libexempi) ベースの python-xmp-toolkit を用い、名前空間宣言や既存プロパティを保持する。
"""

from __future__ import annotations

import os

import _compat  # noqa: F401  # libxmp より前に distutils 互換シムを有効化する

from libxmp import XMPMeta  # noqa: E402
from libxmp.consts import XMP_NS_XMP  # noqa: E402


class XmpError(Exception):
    pass


class XmpWriteError(XmpError):
    """XMP の読み込み・書き込みに失敗 (設計 §9)。当該ファイルのみ失敗として記録。"""


class XmpWriter:
    @staticmethod
    def sidecar_path(raw_path: str) -> str:
        base, _ = os.path.splitext(raw_path)
        return base + ".xmp"

    def write_rating(self, raw_path: str, rating: int) -> None:
        # サイドカーが存在すれば更新モードで開いて Rating のみ上書きし、
        # 無ければ新規作成する (設計 §8.2)。
        xmp_path = self.sidecar_path(raw_path)
        try:
            xmp = XMPMeta()
            if os.path.exists(xmp_path):
                with open(xmp_path, "r", encoding="utf-8") as f:
                    xmp.parse_from_str(f.read())
            xmp.set_property(XMP_NS_XMP, "xmp:Rating", str(rating))
            serialized = xmp.serialize_to_str(omit_packet_wrapper=False)
        except Exception as e:  # libxmp.XMPError 等
            raise XmpWriteError(f"failed to update xmp: {e}") from e
        # 部分書き込みを避けるため一時ファイルへ書いてから置き換える。
        tmp_path = xmp_path + ".tmp"
        try:
            with open(tmp_path, "w", encoding="utf-8") as f:
                f.write(serialized)
            os.replace(tmp_path, xmp_path)
        except OSError as e:
            if os.path.exists(tmp_path):
                os.unlink(tmp_path)
            raise XmpWriteError(f"failed to write xmp: {e}") from e
