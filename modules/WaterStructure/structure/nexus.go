package structure

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/Yeah114/WaterStructure/define"
	"github.com/andybalholm/brotli"
)

const (
	nexusMagic                = "NXUS"
	nexusVersion              = uint8(1)
	nexusCompressionNone      = uint8(0)
	nexusCompressionBrotli    = uint8(1)
	nexusFlagAuthor           = uint8(1 << 0)
	nexusFlagPassword         = uint8(1 << 1)
	nexusSaltSize             = 16
	nexusHashSize             = 32
	nexusMaxAuthorLen         = 4096
)

var (
	ErrNexusPasswordRequired = errors.New("nexus password required")
	ErrNexusPasswordInvalid  = errors.New("nexus password invalid")
)

// Nexus is a compact binary container for MCStructure data.
// Header layout:
//   magic(4) + version(1) + flags(1) + compression(1) + reserved(1)
//   [author_len(uint16) + author_bytes]?
//   [salt(16) + hash(32)]?
//   payload (compressed MCStructure)
type Nexus struct {
	BaseReader
	file     *os.File
	tempFile *os.File
	mc       *MCStructure

	Author   string
	Password string
}

func (n *Nexus) ID() uint8 {
	return IDNexus
}

func (n *Nexus) Name() string {
	return NameNexus
}

func (n *Nexus) FromFile(file *os.File) error {
	if file == nil {
		return ErrInvalidFile
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek nexus failed: %w", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("read nexus magic failed: %w", err)
	}
	if string(header) != nexusMagic {
		return ErrInvalidFile
	}

	meta := make([]byte, 4)
	if _, err := io.ReadFull(file, meta); err != nil {
		return fmt.Errorf("read nexus header failed: %w", err)
	}
	version := meta[0]
	flags := meta[1]
	compression := meta[2]
	if version != nexusVersion {
		return ErrInvalidFile
	}

	if flags&nexusFlagAuthor != 0 {
		authorLen, err := readUint16(file)
		if err != nil {
			return fmt.Errorf("read nexus author length failed: %w", err)
		}
		if authorLen > nexusMaxAuthorLen {
			return ErrInvalidFile
		}
		authorBytes := make([]byte, authorLen)
		if _, err := io.ReadFull(file, authorBytes); err != nil {
			return fmt.Errorf("read nexus author failed: %w", err)
		}
		n.Author = string(authorBytes)
	}

	if flags&nexusFlagPassword != 0 {
		salt := make([]byte, nexusSaltSize)
		if _, err := io.ReadFull(file, salt); err != nil {
			return fmt.Errorf("read nexus salt failed: %w", err)
		}
		hash := make([]byte, nexusHashSize)
		if _, err := io.ReadFull(file, hash); err != nil {
			return fmt.Errorf("read nexus password hash failed: %w", err)
		}
		if strings.TrimSpace(n.Password) == "" {
			return ErrNexusPasswordRequired
		}
		if !checkNexusPassword(n.Password, salt, hash) {
			return ErrNexusPasswordInvalid
		}
	}

	var src io.Reader = file
	switch compression {
	case nexusCompressionNone:
		src = file
	case nexusCompressionBrotli:
		src = brotli.NewReader(file)
	default:
		return ErrInvalidFile
	}

	tempFile, err := os.CreateTemp("", "nexus_mcstructure_*")
	if err != nil {
		return fmt.Errorf("create nexus temp failed: %w", err)
	}

	if _, err := io.Copy(tempFile, src); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return fmt.Errorf("read nexus payload failed: %w", err)
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return fmt.Errorf("seek nexus payload failed: %w", err)
	}

	mc := &MCStructure{}
	if err := mc.FromFile(tempFile); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return err
	}

	n.file = file
	n.tempFile = tempFile
	n.mc = mc
	return nil
}

func (n *Nexus) GetOffsetPos() define.Offset {
	if n.mc == nil {
		return define.Offset{}
	}
	return n.mc.GetOffsetPos()
}

func (n *Nexus) SetOffsetPos(offset define.Offset) {
	if n.mc == nil {
		return
	}
	n.mc.SetOffsetPos(offset)
}

func (n *Nexus) GetSize() define.Size {
	if n.mc == nil {
		return define.Size{}
	}
	return n.mc.GetSize()
}

func (n *Nexus) GetChunks(posList []define.ChunkPos) (map[define.ChunkPos]*chunk.Chunk, error) {
	if n.mc == nil {
		return nil, ErrInvalidFile
	}
	return n.mc.GetChunks(posList)
}

func (n *Nexus) GetChunksNBT(posList []define.ChunkPos) (map[define.ChunkPos]map[define.BlockPos]map[string]any, error) {
	if n.mc == nil {
		return nil, ErrInvalidFile
	}
	return n.mc.GetChunksNBT(posList)
}

func (n *Nexus) CountNonAirBlocks() (int, error) {
	if n.mc == nil {
		return 0, ErrInvalidFile
	}
	return n.mc.CountNonAirBlocks()
}

func (n *Nexus) ToMCWorld(
	bedrockWorld *world.BedrockWorld,
	startSubChunkPos define.SubChunkPos,
	startCallback func(int),
	progressCallback func(),
) error {
	if n.mc == nil {
		return ErrInvalidFile
	}
	return n.mc.ToMCWorld(bedrockWorld, startSubChunkPos, startCallback, progressCallback)
}

func (n *Nexus) FromMCWorld(
	world *world.BedrockWorld,
	target *os.File,
	point1BlockPos define.BlockPos,
	point2BlockPos define.BlockPos,
	startCallback func(int),
	progressCallback func(),
) error {
	if target == nil {
		return fmt.Errorf("nexus target is nil")
	}
	tempFile, err := os.CreateTemp("", "nexus_build_*")
	if err != nil {
		return fmt.Errorf("create nexus temp failed: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	mc := &MCStructure{}
	if err := mc.FromMCWorld(world, tempFile, point1BlockPos, point2BlockPos, startCallback, progressCallback); err != nil {
		return err
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek nexus temp failed: %w", err)
	}

	if _, err := target.Seek(0, io.SeekStart); err == nil {
		_ = target.Truncate(0)
	}

	author := strings.TrimSpace(n.Author)
	password := strings.TrimSpace(n.Password)
	flags := uint8(0)
	if author != "" {
		flags |= nexusFlagAuthor
	}
	if password != "" {
		flags |= nexusFlagPassword
	}

	header := []byte(nexusMagic)
	header = append(header, nexusVersion, flags, nexusCompressionBrotli, 0)
	if _, err := target.Write(header); err != nil {
		return fmt.Errorf("write nexus header failed: %w", err)
	}

	if flags&nexusFlagAuthor != 0 {
		if len(author) > nexusMaxAuthorLen {
			return fmt.Errorf("nexus author too long")
		}
		if err := writeUint16(target, uint16(len(author))); err != nil {
			return fmt.Errorf("write nexus author length failed: %w", err)
		}
		if _, err := target.Write([]byte(author)); err != nil {
			return fmt.Errorf("write nexus author failed: %w", err)
		}
	}

	if flags&nexusFlagPassword != 0 {
		salt := make([]byte, nexusSaltSize)
		if _, err := rand.Read(salt); err != nil {
			return fmt.Errorf("generate nexus salt failed: %w", err)
		}
		hash := hashNexusPassword(password, salt)
		if _, err := target.Write(salt); err != nil {
			return fmt.Errorf("write nexus salt failed: %w", err)
		}
		if _, err := target.Write(hash[:]); err != nil {
			return fmt.Errorf("write nexus password hash failed: %w", err)
		}
	}

	bw := brotli.NewWriter(target)
	if _, err := io.Copy(bw, tempFile); err != nil {
		_ = bw.Close()
		return fmt.Errorf("write nexus payload failed: %w", err)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("finalize nexus payload failed: %w", err)
	}
	return nil
}

func (n *Nexus) Close() error {
	if n.tempFile != nil {
		name := n.tempFile.Name()
		_ = n.tempFile.Close()
		n.tempFile = nil
		if name != "" {
			_ = os.Remove(name)
		}
	}
	return nil
}

func readUint16(r io.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

func writeUint16(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func hashNexusPassword(password string, salt []byte) [nexusHashSize]byte {
	buf := make([]byte, 0, len(salt)+len(password))
	buf = append(buf, salt...)
	buf = append(buf, password...)
	return sha256.Sum256(buf)
}

func checkNexusPassword(password string, salt, hash []byte) bool {
	sum := hashNexusPassword(password, salt)
	if len(hash) != nexusHashSize {
		return false
	}
	return subtle.ConstantTimeCompare(sum[:], hash) == 1
}
