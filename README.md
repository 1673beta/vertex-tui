# vertex-tui

Google GenAI SDK (`google.golang.org/genai`) を利用し、Google Cloud の **Vertex AI (Agent Platform) / Model Garden** のモデルを利用するコーディングエージェント用 TUI です。

## 主な機能

- **Vertex AI Model Garden からのモデル一覧動的取得**
  - TUI 起動時および `/model` 実行時に、Google GenAI SDK (`client.Models.All` / `client.Models.List`) および **Vertex AI Model Garden API (`v1beta1/publishers/{publisher}/models`)** から利用可能なモデル一覧を自動取得します。
  - Google (`publishers/google/models`) だけでなく、Model Garden 上のパートナー／MaaS パブリッシャー (`anthropic`, `meta`, `mistralai`, `deepseek-ai`, `qwen`, `cohere`) やプロジェクト内のチューニング済みモデルにも対応しています。
  - デフォルトではコーディング・対話向け生成 AI モデル (Gemini, Claude, Llama, Codestral/Mistral, DeepSeek, Qwen, Gemma 等) を自動フィルタリングして表示し、`Ctrl+A` または `/model garden all` で非 LLM モデルを含む Model Garden 全モデルの表示にも切り替え可能です。
- **`/model` コマンドによる柔軟なモデル変更**
  - **`/model` (引数なし)** または **`Ctrl+O` / `F2`**: Model Garden 対応の **対話型モデル選択モーダル** を開きます。
    - `↑` / `↓` キーでモデルを選択し、`Enter` で即座に切り替えます。
    - モデル名やパブリッシャー名 (`google`, `anthropic`, `meta`, `mistralai`, `deepseek-ai` 等) でインクリメンタル検索・絞り込みでき、任意の Model Garden モデル ID も直接入力できます。
    - `Ctrl+R`: Vertex AI Model Garden からモデル一覧を再取得します。
    - `Ctrl+A`: コーディング・生成 AI モデルのみ / Model Garden 全モデル表示を切り替えます。
  - **`/model <モデル名|エイリアス|番号>`**: コマンド引数で直接モデルを変更します。
    - 例: `/model gemini-2.5-pro` (または `/model pro`)
    - 例: `/model gemini-2.5-flash` (または `/model flash`)
    - 例: `/model anthropic/claude-3-7-sonnet@20250219` (または `/model claude`)
    - 例: `/model meta/llama-3.3-70b-instruct-maas` (または `/model llama`)
    - 例: `/model mistralai/codestral-2501` (または `/model codestral`)
    - 例: `/model deepseek-ai/deepseek-r1-0528-maas` (または `/model deepseek`)
  - **`/model garden [publisher | all]`**: Vertex AI Model Garden から指定パブリッシャー (例: `/model garden anthropic`, `/model garden google,meta`, `/model garden all`) のモデル一覧を取得します。
  - **`/model list` / `/model current` / `/model refresh`**: モデル一覧表示、現在のモデル詳細確認、Model Garden 再同期を行います。
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

# TUI 起動 (起動時に Model Garden からモデル一覧を自動取得します)
go run .

# CLI から Vertex AI Model Garden のモデル一覧を取得・確認
go run . -list-models
go run . -list-models -publisher google,anthropic,meta
```

## スラッシュコマンド一覧

| コマンド | 説明 |
| --- | --- |
| `/model` | 対話型モデル選択モーダルを開いて Vertex AI / Model Garden モデルを変更 |
| `/model <model-id>` | 指定したモデル ID (または `pro`, `flash`, `claude`, `llama`, `codestral`, 番号) に直接変更 |
| `/model garden [publisher]` | Vertex AI Model Garden (`publishers/{publisher}/models`) からモデル一覧を取得 |
| `/model garden all` | 非 LLM モデルも含めて Model Garden の全モデルを取得 |
| `/model list` | 利用可能な Vertex AI / Model Garden モデル一覧を表示 |
| `/model current` | 現在のモデルの詳細 (Resource Name, Publisher, Launch Stage 等) を表示 |
| `/model refresh` | Vertex AI Model Garden API からモデル一覧を再取得 |
| `/project [id]` | Google Cloud Project ID の確認・変更 |
| `/location [region]` | Vertex AI ロケーション (`us-central1`, `global`, `asia-northeast1` 等) の確認・変更 |
| `/tools` | コーディングエージェントが利用可能なツール一覧を表示 |
| `/status` | Vertex AI 接続状態・Model Garden 同期状態・モデル情報を表示 |
| `/clear` | 会話履歴をクリア |
| `/help` | ヘルプとショートカット一覧を表示 |
| `/exit` (`/quit`) | TUI を終了 |
