# fixPoint 🎯

**FixPoint** is an intelligent debugging assistant that automatically analyzes runtime errors, suggests fixes, and even generates Git commits for them. It bridges the gap between your editor and powerful AI models, turning debugging from a manual chore into a seamless workflow.

## ✨ Key Features

- **AI-Powered Debugging**: Automatically analyzes runtime errors, local variables, and stack traces to understand what went wrong.
- **Smart Fix Suggestions**: Gets context-aware code fixes from LLMs (e.g., Gemini, OpenAI) to resolve issues quickly.
- **Git Commit Generation**: Generates professional commit messages and even drafts the code changes required for the fix.
- **Interactive Workflow**: Catches breakpoints and exceptions, then lets you choose when to engage the AI.
- **Language Agnostic**: Supports any language with a DAP (Debug Adapter Protocol) server.

## 🚀 Quick Start

### Prerequisites

- **Go** (1.21+) installed.
- An API key for a supported LLM:
  - **Gemini API Key**: Required for Google Gemini models.
  - **OpenAI API Key**: Required for OpenAI models.

### Installation

```bash
# Install directly via Go
go install github.com/yourusername/fixpoint/cmd/fixpoint@latest

# OR clone and build manually
git clone https://github.com/yourusername/fixpoint.git
cd fixpoint
go build -o fixpoint ./cmd/fixpoint
```

### Usage

FixPoint runs as a **man-in-the-middle proxy** between your IDE (like VS Code) and your actual debugger (like Go's Delve).

1.  **Configure Your API Key**:
    FixPoint uses a global configuration file, so you only need to set this up once. Run the interactive setup:
    ```bash
    fixpoint config
    ```
    *This will prompt you for your OpenRouter API key and preferred AI model.*

2.  **Start the FixPoint Proxy**:
    ```bash
    fixpoint
    ```
    *The proxy will start and listen on port `4000` by default.*

2.  **Configure Your IDE**:
    - Open your `.vscode/launch.json` (or equivalent).
    - Change your `type` to `pwa` (or `node` for Node.js).
    - Set `request` to `attach`.
    - Crucially, set `port` to `4000` (the default proxy port) and `address` to `localhost`.

    **Example `.vscode/launch.json` for Go (VS Code):**

    ```json
    {
      "version": "0.2.0",
      "configurations": [
        {
          "name": "Attach to FixPoint",
          "type": "go",
          "request": "attach",
          "mode": "remote",
          "port": 4000,
          "host": "127.0.0.1"
        }
      ]
    }
    ```

3.  **Start Debugging**:
    - Run your Go program with `dlv debug --listen=:36281 --headless=true --api-version=2 --accept-multiclient`.
    - Start the "Attach to FixPoint" configuration in VS Code.
    - Hit a breakpoint or exception. FixPoint will automatically capture the context and ask if you want AI analysis.

## 🔧 Configuration

The proxy listens on port `4000` by default, forwarding traffic to `36281`.

You can customize these ports (useful if 4000 is taken):

```bash
# Listen on 5000, forward to 36281
fixpoint -listen :5000 -debugger :36281

# Enable verbose logging (raw DAP protocol messages)
fixpoint -verbose
```

## 🔄 How It Works

1.  **Listen**: FixPoint starts a TCP server on port 4000.
2.  **Proxy**: It forwards all traffic between your IDE and the Debug Adapter (e.g., delve).
3.  **Intercept**: When the Debug Adapter sends a `stopped` event (breakpoint hit, exception), FixPoint intercepts it.
4.  **Analyze**: It captures:
    - Stack trace
    - Local variables
    - Exception details
    - The source code snippet around the hit line
5.  **Assist**: It sends this context to the configured LLM.
6.  **Respond**:
    - For **errors/exceptions**: It immediately shows the AI's suggested fix.
    - For **breakpoints**: It shows the variables and waits for you to press `Enter` to ask for a fix.

## 🛠️ Developing

If you want to contribute or modify the behavior:

- The main entry point is `cmd/fixpoint/main.go`.
- Global configuration is handled in `config/config.go`.
- The core proxy logic is in `proxy.go`.
- AI interaction happens in `ai.go`.
- Context capture logic is in `interrogator.go`.

## 📄 License

MIT License
