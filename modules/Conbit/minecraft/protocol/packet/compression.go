package packet

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/LangTuStudio/Conbit/minecraft/internal"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/flate"
)

// Compression represents a compression algorithm that can compress and decompress data.
type Compression interface {
	// EncodeCompression encodes the compression algorithm into a uint16 ID.
	EncodeCompression() uint16
	// Compress compresses the given data and returns the compressed data.
	Compress(decompressed []byte) ([]byte, error)
	// Decompress decompresses the given data and returns the decompressed data.
	// If limit is greater than 0, it limits the decompressed size.
	Decompress(compressed []byte, limit ...int) ([]byte, error)
}

// nopCompression is an empty implementation that does not compress data.
type nopCompression struct{}

// neteaseCompression is the implementation of the NetEase (Fixed Flate) compression algorithm.
type neteaseCompression struct {
	// dict is the sliding window used when decompressing flate,
	// and its size should always be kept within 32KB (32768 Bytes)
	dict []byte
	mu   sync.Mutex
}

// EncodeCompression ...
func (*nopCompression) EncodeCompression() uint16 {
	return CompressionAlgorithmNone
}

// Compress ...
func (*nopCompression) Compress(decompressed []byte) ([]byte, error) {
	return decompressed, nil
}

// Decompress ...
func (*nopCompression) Decompress(compressed []byte, limit ...int) ([]byte, error) {
	return compressed, nil
}

// EncodeCompression ...
func (*neteaseCompression) EncodeCompression() uint16 {
	return CompressionAlgorithmNetEase
}

// Compress ...
func (n *neteaseCompression) Compress(decompressed []byte) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Get new buffer and writer.
	compressed := internal.BufferPool.Get().(*bytes.Buffer)
	w := flateCompressPool.Get().(*flate.Writer)

	defer func() {
		// Reset the buffer, so we can return it to the buffer pool safely.
		compressed.Reset()
		internal.BufferPool.Put(compressed)
		flateCompressPool.Put(w)
	}()

	w.Reset(compressed)

	// Write data to writer and flush.
	if _, err := w.Write(decompressed); err != nil {
		return nil, fmt.Errorf("netease compression: write flate writer: %w", err)
	}
	if err := w.Flush(); err != nil {
		return nil, fmt.Errorf("netease compression: flush flate writer: %w", err)
	}
	// Note that NetEase only accept a flate block
	// that is have no end tag, so before we close
	// the writer, we need to record current len of
	// compressed.
	validLen := compressed.Len()
	// Now we can close the writer safely.
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("netease compression: close flate writer: %w", err)
	}

	return append([]byte{}, compressed.Bytes()[:validLen]...), nil
}

// Decompress ...
func (n *neteaseCompression) Decompress(compressed []byte, limit ...int) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Create buffer.
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	c := flateDecompressPool.Get().(io.ReadCloser)

	defer func() {
		// Reset the buffer, so we can return it to the buffer pool safely.
		buf.Reset()
		internal.BufferPool.Put(buf)
		flateDecompressPool.Put(c)
	}()

	// The given compressed data is not completely,
	// so we need to add the suffix manually.
	buf.Write(compressed)
	buf.Write([]byte{0x01, 0x00, 0x00, 0xFF, 0xFF})

	// Do decompress.
	if err := c.(flate.Resetter).Reset(buf, n.dict); err != nil {
		return nil, fmt.Errorf("netease decompression: reset flate: %w", err)
	}
	_ = c.Close()

	// Guess an uncompressed size of 2*len(compressed).
	decompressed := bytes.NewBuffer(make([]byte, 0, len(compressed)*2))
	if _, err := io.Copy(decompressed, c); err != nil {
		return nil, fmt.Errorf("netease decompression: decompress flate: %w", err)
	}

	// Update dict so that the next compressed
	// block can refer data from the history.
	maxWinLen := 32768
	decompLen := decompressed.Len()
	mergedLen := len(n.dict) + decompLen

	if mergedLen <= maxWinLen {
		n.dict = append(n.dict, decompressed.Bytes()...)
	} else {
		newDict := make([]byte, maxWinLen)
		if decompLen >= maxWinLen {
			copy(newDict, decompressed.Bytes()[decompLen-maxWinLen:])
		} else {
			// Note that you can prove oldKeep is always lower than len(n.dict).
			oldKeep := maxWinLen - decompLen
			copy(newDict[:oldKeep], n.dict[len(n.dict)-oldKeep:])
			copy(newDict[oldKeep:], decompressed.Bytes())
		}
		n.dict = newDict
	}

	return decompressed.Bytes(), nil
}

var (
	// FlateCompression is the implementation of the Flate compression
	// algorithm. This was used by default until v1.19.30.
	FlateCompression = func() Compression { return &flateCompression{} }
	// SnappyCompression is the implementation of the Snappy compression
	// algorithm. This is used by default.
	SnappyCompression = func() Compression { return &snappyCompression{} }
	// NopCompression is an empty implementation that does not compress data.
	NopCompression = func() Compression { return &nopCompression{} }
	// NeteaseCompression is the implementation of the NetEase (Fixed Flate)
	// compression algorithm. This is used by NetEase Rental Server.
	NeteaseCompression = func() Compression { return &neteaseCompression{} }

	DefaultCompression func() Compression = NeteaseCompression
)

// flateCompression is the implementation of the Flate compression algorithm. This was used by default until v1.19.30.
type flateCompression struct{}

// snappyCompression is the implementation of the Snappy compression algorithm. This is used by default.
type snappyCompression struct{}

// flateDecompressPool is a sync.Pool for io.ReadCloser flate readers. These are
// pooled for connections.
var (
	flateDecompressPool = sync.Pool{
		New: func() any { return flate.NewReader(bytes.NewReader(nil)) },
	}
	flateCompressPool = sync.Pool{
		New: func() any {
			w, _ := flate.NewWriter(io.Discard, 6)
			return w
		},
	}
)

// EncodeCompression ...
func (flateCompression) EncodeCompression() uint16 {
	return 0
}

// Compress ...
func (flateCompression) Compress(decompressed []byte) ([]byte, error) {
	compressed := internal.BufferPool.Get().(*bytes.Buffer)
	w := flateCompressPool.Get().(*flate.Writer)

	defer func() {
		// Reset the buffer, so we can return it to the buffer pool safely.
		compressed.Reset()
		internal.BufferPool.Put(compressed)
		flateCompressPool.Put(w)
	}()

	w.Reset(compressed)

	_, err := w.Write(decompressed)
	if err != nil {
		return nil, fmt.Errorf("compress flate: %w", err)
	}
	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("close flate writer: %w", err)
	}
	return append([]byte(nil), compressed.Bytes()...), nil
}

// Decompress ...
func (flateCompression) Decompress(compressed []byte, limit ...int) ([]byte, error) {
	// Handle size limit if specified
	if len(limit) > 0 && limit[0] > 0 {
		// For flate, we can't easily check the uncompressed size beforehand
		// but we can apply a reader limit during decompression
	}

	buf := bytes.NewReader(compressed)
	c := flateDecompressPool.Get().(io.ReadCloser)
	defer flateDecompressPool.Put(c)

	if err := c.(flate.Resetter).Reset(buf, nil); err != nil {
		return nil, fmt.Errorf("reset flate: %w", err)
	}
	_ = c.Close()

	// Apply limit if specified
	reader := io.Reader(c)
	if len(limit) > 0 && limit[0] > 0 {
		reader = io.LimitReader(c, int64(limit[0]))
	}

	// Guess an uncompressed size of 2*len(compressed).
	decompressed := bytes.NewBuffer(make([]byte, 0, len(compressed)*2))
	if _, err := io.Copy(decompressed, reader); err != nil {
		return nil, fmt.Errorf("decompress flate: %v", err)
	}
	return decompressed.Bytes(), nil
}

// EncodeCompression ...
func (snappyCompression) EncodeCompression() uint16 {
	return 1
}

// Compress ...
func (snappyCompression) Compress(decompressed []byte) ([]byte, error) {
	// Because Snappy allocates a slice only once, it is less important to have
	// a dst slice pre-allocated. With flateCompression this is more important,
	// because flate does a lot of smaller allocations which causes a
	// considerable slowdown.
	return snappy.Encode(nil, decompressed), nil
}

// Decompress ...
func (snappyCompression) Decompress(compressed []byte, limit ...int) ([]byte, error) {
	// Handle size limit if specified
	if len(limit) > 0 && limit[0] > 0 {
		decodedLen, err := snappy.DecodedLen(compressed)
		if err != nil {
			return nil, fmt.Errorf("snappy decoded length: %w", err)
		}
		if decodedLen > limit[0] {
			return nil, fmt.Errorf("snappy decoded size %d exceeds limit %d", decodedLen, limit[0])
		}
	}

	// Snappy writes a decoded data length prefix, so it can allocate the
	// perfect size right away and only needs to allocate once. No need to pool
	// byte slices here either.
	decompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		return nil, fmt.Errorf("decompress snappy: %w", err)
	}
	return decompressed, nil
}

// init registers all valid compressions with the protocol.
func init() {
	RegisterCompression(FlateCompression)
	RegisterCompression(SnappyCompression)
	// Register additional compression algorithms
	RegisterCompression(NopCompression)
	RegisterCompression(NeteaseCompression)
}

var (
	compressions  = map[uint16]Compression{}
	compressFuncs = map[uint16]func() Compression{}
)

// RegisterCompression registers a compression so that it can be used by the protocol.
func RegisterCompression(compressFunc func() Compression) {
	compression := compressFunc()
	compressions[compression.EncodeCompression()] = compression
	compressFuncs[compression.EncodeCompression()] = compressFunc
}

// CompressionByID attempts to return a compression by the ID it was registered with. If found, the compression found
// is returned and the bool is true.
func CompressionByID(id uint16) (Compression, bool) {
	c, ok := compressions[id]
	if !ok {
		c = DefaultCompression()
	}
	return c, ok
}

// CompressFuncByID attempts to return a func that return a new(compression) by the ID it was registered with.
// If found, the compression found is returned and the bool is true.
func CompressFuncByID(id uint16) (func() Compression, bool) {
	c, ok := compressFuncs[id]
	if !ok {
		c = DefaultCompression
	}
	return c, ok
}
