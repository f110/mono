# UI 規約

自作ツール（Issue, Codexel など）の Web フロントエンドで共通して守る UI の規約。
ツールごとの設計は各ツールのドキュメントに書き、ツールをまたいで揃えるべき挙動だけをここに書く。

実装は各ツールで個別に持ち、共有パッケージにはしない。

## テキスト入力の送信

複数行のテキスト入力（コメント、本文、プロンプトなど）を送信・保存するキーは次のとおり。

| キー                         | 動作    |
|----------------------------|-------|
| Cmd+Enter (macOS)          | 送信・保存 |
| Ctrl+Enter (Windows/Linux) | 送信・保存 |
| Enter                      | 改行    |

判定は `e.key === 'Enter' && (e.metaKey || e.ctrlKey)` とする。OS を判別せずに両方を受け付ける。

### IME 変換中の扱い

IME で変換中のキー入力では送信しない。変換の確定に使ったキー入力も変換中として扱う。

判定は MDN の推奨に従い、`isComposing` と `keyCode === 229` の両方を見る。

```ts
if (e.isComposing || e.keyCode === 229) {
  return
}
```

- `isComposing` は UI Events で「`compositionstart` の後、対応する `compositionend` の前に発生したキーイベント」で
  `true` になると定義されている
- ブラウザによっては変換を確定するキーの `keydown` が `compositionend` の後に発生し（Safari など）、
  そのイベントでは `isComposing` が `false` になる。変換を開始するキーでも同様に `compositionstart` より前に `keydown` が発生することがある
- UI Events の legacy な `keyCode` の決定手順では、IME がキー入力を処理している `keydown` の `keyCode` は 229 になる。
  `keyCode` は非推奨だが、上記のケースを判定する手段がこれしかないため併用する
- React の `onKeyDown` では `e.nativeEvent` に対して判定する

参考

- [UI Events - KeyboardEvent.isComposing](https://w3c.github.io/uievents/#dom-keyboardevent-iscomposing)
- [UI Events - How to determine keyCode for keydown and keyup events](https://w3c.github.io/uievents/#determine-keydown-keyup-keyCode)
- [MDN - Element: keydown event](https://developer.mozilla.org/en-US/docs/Web/API/Element/keydown_event#keydown_events_with_ime)
