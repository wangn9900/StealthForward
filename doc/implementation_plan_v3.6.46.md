
**Core Issue**
The user pointed out that "AnyTLS" should not use `flow: xtls-rprx-vision`.
Although AnyTLS is technically VLESS+TCP+TLS, in this context (StealthForward), it implies a pure TLS configuration without the XTLS Vision flow control, likely for compatibility or specific network characteristics.
The previous implementation treated AnyTLS identically to VLESS, injecting `xtls-rprx-vision` by default for TCP transport.

**Fix Implemented**
1.  **Explicit AnyTLS Handling**: Modified `internal/generator/singbox_config.go`'s `generateUsers` function to explicitly handle the `anytls` protocol case. In this case, it generates the user config with `uuid` and `name` but strictly omits the `flow` field.
2.  **Protocol Type Separation**: Refactored the configuration generation logic to distinguish between the "Sing-box Protocol Type" (which must be `vless` for AnyTLS) and the "Functional Protocol Name" (which is `anytls`).
    *   This ensures `sing-box` receives the correct `type: vless` configuration.
    *   BUT the user generator receives `anytls` to know it should skip flow generation.
3.  **Mapped Inbound Fix**: Applied the same logic to Mapped Inbounds (multi-port), ensuring if a mapping is set to "AnyTLS", it also gets converted to `vless` for sing-box but generates users without flow. This also fixed a potential bug where mapped inbounds might have sent `type: anytls` to sing-box (which would fail).

**Verification**
- Bumped Agent version to `v3.6.46 (AnyTLS No-Flow Fix)`.
- Recompiled the Linux agent binary.

**Next Steps**
- Deploy `stealth-agent-linux-amd64` to the node.
- Verify that `sing-box` starts without errors and that clients can connect (clients should likely disable flow/reality settings if they were using AnyTLS).
