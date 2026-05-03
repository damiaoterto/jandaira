#!/bin/bash

# Default values if no arguments are provided
SANDBOX_NAME="${1:-mcp-base}"
WORKSPACE_PATH="${2:-.}"

echo "[INFO] Starting sandbox verification for: $SANDBOX_NAME"

# 1. Check if sbx CLI is installed
if ! command -v sbx &> /dev/null; then
    echo "[ERROR] 'sbx' CLI could not be found. Please install it first."
    exit 1
fi

# 2. Check if the sandbox already exists
# We do this by trying to run a dummy command ('true') inside it
echo "[INFO] Checking if sandbox '$SANDBOX_NAME' exists..."

if sbx exec "$SANDBOX_NAME" true > /dev/null 2>&1; then
    echo "[INFO] Sandbox '$SANDBOX_NAME' already exists and is ready to use."
else
    echo "[WARN] Sandbox '$SANDBOX_NAME' not found or not responding."
    echo "[INFO] Creating a new sandbox '$SANDBOX_NAME' in workspace path '$WORKSPACE_PATH'..."
    
    # 3. Create the sandbox using the 'shell' template
    if sbx create --name "$SANDBOX_NAME" shell "$WORKSPACE_PATH"; then
        echo "[SUCCESS] Sandbox '$SANDBOX_NAME' created successfully!"
    else
        echo "[ERROR] Failed to create sandbox '$SANDBOX_NAME'. Check the sbx output above."
        exit 1
    fi
fi

# 4. Pre-provision npm global prefix directory so npx can install packages
# without a shell wrapper. npm lstat's its prefix/lib dir on startup and
# fails with ENOENT if it does not exist. We create it once here so every
# subsequent "sbx exec -i mcp-base npx ..." call works directly.
echo "[INFO] Configuring npm global prefix in sandbox..."
sbx exec "$SANDBOX_NAME" sh -c \
    "mkdir -p /usr/local/share/npm-global/lib && npm config set prefix /usr/local/share/npm-global" \
    && echo "[INFO] npm global prefix configured." \
    || echo "[WARN] npm prefix setup failed; node.js MCP servers may not work."

# 5. Final verification
echo "[INFO] Testing connectivity with the sandbox..."
if sbx exec "$SANDBOX_NAME" echo "Connection established."; then
    echo "[SUCCESS] Sandbox is ready. You can now execute MCP servers."
else
    echo "[ERROR] Sandbox was created but test command failed."
    exit 1
fi