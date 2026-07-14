package packet

const (
	CompressionAlgorithmFlate = iota
	CompressionAlgorithmSnappy
	CompressionAlgorithmNetEase
	CompressionAlgorithmNone = 0xffff
)
