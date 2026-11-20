# vertex-tui

Google GenAI SDK (`google.golang.org/genai`) を利用し、Google Cloud の **Vertex AI (Agent Platform)** のモデルを利用するコーディングエージェント用 TUI です。

## 主な機能

- **Google GenAI SDK (`google.golang.org/genai`) + Vertex AI バックエンド (`genai.BackendVertexAI`)**
  - Google Cloud の Application Default Credentials (ADC) および `GOOGLE_CLOUD_PROJECT` / `GOOGLE_CLOUD_LOCATION` を使用して Vertex AI モデルに接続します。
  - ストリーミング応答 (`GenerateContentStream`) および思考プロセス (`Thinking`) のリアルタイム表示に対応しています。
- **`/model` コマンドによる柔軟なモデル変更**
  - **`/model` (引数なし)** または **`Ctrl+O` / `F2`**: TUI 内で対話型の **モデル選択モーダル** を開きます。
    - `↑` / `↓` キーでモデルを選択し、`Enter` で即座に切り替えます。
    - 文字を入力するとモデル一覧をインクリメンタル検索・絞り込みでき、任意のカスタム Vertex AI モデル ID も直接指定できます。
    - `Ctrl+R` で Vertex AI API (`client.Models.All`) から利用可能なモデル一覧を動的に取得・更新できます。
  - **`/model <モデル名|エイリアス|番号>`**: コマンド引数で直接モデルを変更します。
    - 例: `/model gemini-2.5-pro`
    - 例: `/model gemini-2.5-flash` (または `/model flash`)
    - 例: `/model gemini-2.5-flash-lite` (または `/model flash-lite`)
    - 例: `/model gemini-2.0-flash`
    - 例: `/model 2` (`/model list` の番号指定)
  - **`/model list`**: 利用可能なモデル一覧と現在選択中のモデルを表示します。
  - **`/model current`**: 現在選択中のモデルの詳細情報を表示します。
  - **`/model refresh`**: Vertex AI からモデル一覧を再取得します。
  - 会話履歴を保持したままモデルを切り替えられるため、タスクの途中で Flash と Pro を使い分けることができます。
- **コーディングエージェント用ツール (Vertex AI Function Calling)**
  - `read_file`: ワークスペース内のファイルを行番号付きで読み取り (行範囲指定対応)
  - `write_file`: ファイルの新規作成・上書き保存
  - `edit_file`: 既存ファイル内の指定テキスト置換
  - `list_files`: ワークスペース内のディレクトリ・ファイルツリー取得
  - `search_files`: ワークスペース内の横断テキスト検索
  - `run_command`: ワークスペース内でのシェルコマンド・テスト実行

## セットアップと起動

```bash
# Google Cloud 認証 (ADC) とプロジェクト設定
gcloud auth application-default login
export GOOGLE_CLOUD_PROJECT="your-gcp-project-id"
export GOOGLE_CLOUD_LOCATION="us-central1"

# 起動
go run .

# またはフラグで初期モデル・プロジェクトを指定して起動
go run . -project your-gcp-project-id -location us-central1 -model gemini-2.5-pro
```

## スラッシュコマンド一覧

| コマンド | 説明 |
| --- | --- |
| `/model` | 対話型モデル選択モーダルを開いて Vertex AI モデルを変更 |
| `/model <model-id>` | 指定したモデル ID (または `pro`, `flash`, `flash-lite`, 番号) に直接変更 |
| `/model list` | 利用可能な Vertex AI モデル一覧を表示 |
| `/model current` | 現在のモデルの詳細を表示 |
| `/model refresh` | Vertex AI API から利用可能なモデル一覧を取得・更新 |
| `/project [id]` | Google Cloud Project ID の確認・変更 |
| `/location [region]` | Vertex AI ロケーション (`us-central1`, `global`, `asia-northeast1` 等) の確認・変更 |
| `/tools` | コーディングエージェントが利用可能なツール一覧を表示 |
| `/status` | Vertex AI 接続状態・現在のモデル・ワークスペース情報を表示 |
| `/clear` | 会話履歴をクリア |
| `/help` | ヘルプとショートカット一覧を表示 |
| `/exit` (`/quit`) | TUI を終了 |
