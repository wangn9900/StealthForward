
**Core Issue**
The `sing-box` configuration generator (`singbox_config.go`) was pre-generating user configuration objects (including authentication fields like `uuid` or `password`) based solely on the Entry Node's primary protocol. However, these user objects were then reused for additional "Mapped Inbounds" (multi-port mappings) which might use different protocols.

Specifically, if the Entry Node was configured as **Trojan** (generating `password`), but a mapped inbound was configured as **AnyTLS/VLESS** (expecting `uuid` and rejecting `password`), `sing-box` would fail to start with the error:
`unmarshal config error: inbounds[1].users[0].password: json: unknown field "password"`

**Fix Implemented**
1.  **Refactored Data Storage**: Changed `portToUsers` to store raw `models.ForwardingRule` structs instead of pre-generated map objects.
2.  **Dynamic User Generation**: Created a helper function `generateUsers(protocol, rules)` inside `GenerateEntryConfig`. this function generates the correct user configuration schema (UUID for VLESS/VMess, Password for Trojan/Shadowsocks) on-demand for each specific inbound based on its actual protocol type.
3.  **Application**: applied this dynamic generation to both the default inbound (Entry Protocol) and all mapped inbounds.

**Verification**
- Reviewed the refactored loop and logic in `internal/generator/singbox_config.go` to ensure all fields (Flow, Name) are correctly handled.
- Bumped Agent version to `v3.6.45 (AnyTLS Mapping Fix)` in `internal/agent/agent.go` to help identify the fix.
- Initiated a build check to ensure no syntax errors were introduced.

**Next Steps**
- Please compile the agent (`make build-agent` or similar) and deploy `stealth-agent-linux-amd64` to your test node.
- Verify that the error is resolved and the agent starts successfully.
