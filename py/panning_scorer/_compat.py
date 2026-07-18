"""Python 3.12 で標準ライブラリから削除された distutils を補う互換シム。

python-xmp-toolkit 2.0.1 の libxmp/version.py が distutils.version.LooseVersion に
依存しているため、libxmp をインポートする前にこのモジュールを読み込む。
setuptools(>=60) を import すると同梱 distutils のシムが有効になり、以降
`import distutils` が解決できる (setuptools 公式の推奨手順)。
"""

try:
    import distutils.version  # noqa: F401
except ModuleNotFoundError:  # Python 3.12+
    import setuptools  # noqa: F401  # distutils シムを有効化する副作用のために import する
    import distutils.version  # noqa: F401
