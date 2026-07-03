package installer

// Well-known State keys shared between step packages (windows, mac) and the
// install command, so values produced by one step can be consumed by another.
const (
	KeySystemInfo       = "system_info"      // platform.SystemInfo from preflight
	KeyServerBinary     = "llama_binary"     // path to the acquired llama-server
	KeyModelPath        = "model_path"       // path to the downloaded model file
	KeyServerConfigPath = "wg_server_config" // path to the server's wg config
	KeyClientConfigPath = "wg_client_config" // path to the client's wg config
	KeyJoinToken        = "join_token"       // base64 join token minted by server
	KeyEnrollReply      = "enroll_reply"     // base64 enrollment reply from client
)
