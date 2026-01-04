# Encryption Smart Key Implementation Plan

## Overview
Implemented a "Smart Key" system (`STEALTH-` prefix) that embeds the License Server URL within an encrypted payload. This allows clients (Controllers) to automatically connect to the correct authorization server without manual IP configuration, enhancing user experience and hiding server infrastructure details.

## Components

### 1. License Server (Authorization Center)
**Status**: Upgraded
- **Logic**: Updated `createLicenseHandler` to accept a `server_url` (auto-detected from browser origin).
- **Crypto**: Implemented AES-256-GCM encryption with a shared `SmartKeySecret`.
- **Output**: Generates a standard `LicenseKey` AND a `SmartKey`.
- **UI**: 
  - Admin page now automatically detects the current URL.
  - "Create" button generates the Smart Key.
  - Alert popup displays the Smart Key for copying.

### 2. StealthForward Controller (Client)
**Status**: Upgraded
- **Logic**: Updated `internal/license` module.
- **Parsing**: `SetKey` and `Init` functions now detect `STEALTH-` prefix.
- **Decryption**: Decodes Base64 and decrypts AES payload to extract `server_url` and `license_key`.
- **Auto-Config**: Automatically sets the internal `serverURL` variable upon key entry, overriding defaults.
- **Persistence**: The Smart Key is saved to `data/license.key`, ensuring the server configuration persists across restarts without a separate config file entry.

### 3. Frontend (Web UI)
**Status**: Updated
- **Header**: Updated activation input field width (`w-96`) and placeholder to accommodate longer Smart Key strings.

## Workflow
1. **Admin** logs into License Server.
2. **Admin** clicks "Create License". The system packs the current server URL into the key.
3. **Admin** sends the **Smart Key** to the Customer.
4. **Customer** pastes the Smart Key into StealthForward Web UI.
5. **StealthForward** backend decrypts the URL and Key.
6. **StealthForward** connects to the hidden URL and activates the license.

## Security
- **Algorithm**: AES-256-GCM.
- **Key**: Hardcoded shared secret `StealthForward_Smart_License_Key_2025_Secret` in binary.
- **Privacy**: Server IP is not displayed in the client UI and is obfuscated in the key string.

## Verification
- **Builds**: Validated that both `stealth-controller` and `license-server` compile successfully.
