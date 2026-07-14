package starshuttler_runtime

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"sync"
	"time"
	"unsafe"

	NexusGameInterface "nexus/utils/api/game_interface"
	"nexus/utils/log"

	nexusprotocol "github.com/LangTuStudio/Conbit/minecraft/protocol"
	nexuspacket "github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	starclient "github.com/LangTuStudio/RaaBel/client"
	starminecraft "github.com/LangTuStudio/RaaBel/core/minecraft"
	starprotocol "github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	starlogin "github.com/LangTuStudio/RaaBel/core/minecraft/protocol/login"
	starpacket "github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	stargameinterface "github.com/LangTuStudio/RaaBel/game_control/game_interface"
	starresources "github.com/LangTuStudio/RaaBel/game_control/resources_control"
	starassigner "github.com/LangTuStudio/RaaBel/nbt_assigner"
	starcache "github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	starconsole "github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	starutils "github.com/LangTuStudio/RaaBel/utils"
)

//go:linkname newStarConn github.com/LangTuStudio/RaaBel/core/minecraft.newConn
func newStarConn(net.Conn, *ecdsa.PrivateKey, *slog.Logger, starminecraft.Protocol, time.Duration, bool) *starminecraft.Conn

//go:linkname handleStarPacket github.com/LangTuStudio/RaaBel/game_control/resources_control.(*Resources).handlePacket
func handleStarPacket(*starresources.Resources, starpacket.Packet)

//go:linkname updateStarConstantByGameData github.com/LangTuStudio/RaaBel/game_control/resources_control.(*ConstantPacket).updateByGameData
func updateStarConstantByGameData(*starresources.ConstantPacket, starminecraft.GameData)

type Runtime struct {
	mainInterface *NexusGameInterface.GameInterface
	conn          *starminecraft.Conn
	client        *starclient.Client
	resources     *starresources.Resources
	api           *stargameinterface.GameInterface

	mu        sync.Mutex
	console   *starconsole.Console
	assigner  *starassigner.NBTAssigner
	dimension uint8
	center    [3]int32
}

type PlaceResult struct {
	CanFast       bool
	StructureName string
	Offset        [3]int32
}

var runtimes sync.Map

func GetOrCreate(intf *NexusGameInterface.GameInterface) (*Runtime, error) {
	if intf == nil {
		return nil, fmt.Errorf("starshuttler runtime: game interface unavailable")
	}
	if value, ok := runtimes.Load(intf); ok {
		return value.(*Runtime), nil
	}

	runtime, err := newRuntime(intf)
	if err != nil {
		return nil, err
	}
	actual, _ := runtimes.LoadOrStore(intf, runtime)
	return actual.(*Runtime), nil
}

func FeedPacket(intf *NexusGameInterface.GameInterface, pk nexuspacket.Packet) {
	if intf == nil || pk == nil {
		return
	}
	value, ok := runtimes.Load(intf)
	if !ok {
		return
	}
	runtime := value.(*Runtime)
	starPK, err := nexusPacketToStar(pk)
	if err != nil {
		return
	}
	if itemRegistry, ok := pk.(*nexuspacket.ItemRegistry); ok {
		runtime.updateItemRegistry(itemRegistry)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Log.Warn(fmt.Sprintf("RaaBel packet feed skipped: %v", recovered))
		}
	}()
	handleStarPacket(runtime.resources, starPK)
}

func (r *Runtime) PlaceNBTBlock(
	blockName string,
	blockStates map[string]any,
	blockNBT map[string]any,
	dimensionID uint8,
	center [3]int32,
) (PlaceResult, error) {
	if r == nil {
		return PlaceResult{}, fmt.Errorf("starshuttler runtime unavailable")
	}
	assigner, err := r.ensureAssigner(dimensionID, center)
	if err != nil {
		return PlaceResult{}, err
	}
	canFast, uniqueID, offset, err := assigner.PlaceNBTBlock(blockName, blockStates, blockNBT)
	if err != nil {
		return PlaceResult{}, err
	}
	result := PlaceResult{
		CanFast: canFast,
		Offset:  [3]int32{offset[0], offset[1], offset[2]},
	}
	if !canFast {
		result.StructureName = starutils.MakeUUIDSafeString(uniqueID)
	}
	return result, nil
}

func (r *Runtime) RestoreConsole() error {
	return nil
}

func newRuntime(intf *NexusGameInterface.GameInterface) (*Runtime, error) {
	proxy := newPacketProxyConn()
	conn := newStarConn(proxy, nil, slog.Default(), starminecraft.DefaultProtocol, 50*time.Millisecond, false)
	runtime := &Runtime{
		mainInterface: intf,
		conn:          conn,
	}
	proxy.write = func(_ []byte) (int, error) {
		return 0, nil
	}
	packetForwarder := func(header starpacket.Header, payload []byte, _, _ net.Addr) {
		pk, err := starWirePacketToNexus(header.PacketID, payload)
		if err != nil {
			log.Log.Warn(fmt.Sprintf("RaaBel packet convert failed: %v", err))
			return
		}
		if err := intf.WritePacket(pk); err != nil {
			log.Log.Warn(fmt.Sprintf("RaaBel packet send failed: %v", err))
		}
	}
	setPrivateField(conn, "packetFunc", packetForwarder)
	setPrivateField(conn, "identityData", convertIdentityData(intf.ClientInfo))
	setPrivateField(conn, "gameData", convertGameDataFromInterface(intf))

	client := &starclient.Client{}
	cached := make(chan starpacket.Packet)
	close(cached)
	setPrivateField(client, "connection", conn)
	setPrivateField(client, "cachedPacket", cached)

	resources := starresources.NewResourcesControl(client)
	runtime.client = client
	runtime.resources = resources
	api := stargameinterface.NewGameInterface(resources)
	runtime.api = api
	runtime.syncConstantFromNexus()
	return runtime, nil
}

func (r *Runtime) ensureAssigner(dimensionID uint8, center [3]int32) (*starassigner.NBTAssigner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.assigner != nil && r.dimension == dimensionID && r.center == center {
		return r.assigner, nil
	}
	console, err := starconsole.NewConsole(
		r.api,
		dimensionID,
		starprotocol.BlockPos{center[0], center[1], center[2]},
	)
	if err != nil {
		return nil, fmt.Errorf("RaaBel NewConsole: %v", err)
	}
	cache := starcache.NewNBTCacheSystem(console)
	r.console = console
	r.assigner = starassigner.NewNBTAssigner(console, cache)
	r.dimension = dimensionID
	r.center = center
	return r.assigner, nil
}

func (r *Runtime) updateItemRegistry(p *nexuspacket.ItemRegistry) {
	if r == nil || r.resources == nil || p == nil {
		return
	}
	data := r.conn.GameData()
	data.Items = convertItemEntries(p.Items)
	updateStarConstantByGameData(r.resources.ConstantPacket(), data)
	setPrivateField(r.conn, "gameData", data)
}

func (r *Runtime) syncConstantFromNexus() {
	if r == nil || r.mainInterface == nil || r.mainInterface.Resources == nil || r.resources == nil {
		return
	}
	source := r.mainInterface.Resources.ConstantPacket()
	if source == nil {
		return
	}
	items := source.AllAvailableItems()
	if len(items) > 0 {
		data := r.conn.GameData()
		data.Items = convertItemEntries(items)
		updateStarConstantByGameData(r.resources.ConstantPacket(), data)
		setPrivateField(r.conn, "gameData", data)
	}
	commandItems := source.AllCommandItems()
	if len(commandItems) == 0 {
		commandItems = []string{"minecraft:written_book"}
		for _, item := range items {
			commandItems = append(commandItems, item.Name)
		}
	}
	commandMapping := make(map[string]bool, len(commandItems))
	for _, itemName := range commandItems {
		commandMapping[itemName] = true
	}
	constant := r.resources.ConstantPacket()
	setPrivateField(constant, "commandItems", commandItems)
	setPrivateField(constant, "commandItemsMapping", commandMapping)
}

func convertIdentityData(info NexusGameInterface.ClientInfo) starlogin.IdentityData {
	return starlogin.IdentityData{
		XUID:        info.XUID,
		Identity:    info.ClientIdentity,
		DisplayName: info.DisplayName,
	}
}

func convertGameDataFromClientInfo(info NexusGameInterface.ClientInfo) starminecraft.GameData {
	return starminecraft.GameData{
		EntityUniqueID:  info.EntityUniqueID,
		EntityRuntimeID: info.EntityRuntimeID,
	}
}

func convertGameDataFromInterface(intf *NexusGameInterface.GameInterface) starminecraft.GameData {
	if intf == nil {
		return starminecraft.GameData{}
	}
	data := convertGameDataFromClientInfo(intf.ClientInfo)
	if intf.Resources == nil {
		return data
	}
	source := intf.Resources.ConstantPacket()
	if source == nil {
		return data
	}
	data.Items = convertItemEntries(source.AllAvailableItems())
	return data
}

func convertItemEntries(items []nexusprotocol.ItemEntry) []starprotocol.ItemEntry {
	result := make([]starprotocol.ItemEntry, 0, len(items))
	for _, item := range items {
		result = append(result, starprotocol.ItemEntry{
			Name:           item.Name,
			RuntimeID:      item.RuntimeID,
			ComponentBased: item.ComponentBased,
			Version:        item.Version,
			Data:           item.Data,
		})
	}
	return result
}

func nexusPacketToStar(pk nexuspacket.Packet) (starpacket.Packet, error) {
	raw, err := encodeNexusPacket(pk)
	if err != nil {
		return nil, err
	}
	return decodeStarPacket(raw)
}

func starWirePacketToNexus(packetID uint32, payload []byte) (nexuspacket.Packet, error) {
	buffer := bytes.NewBuffer(nil)
	header := nexuspacket.Header{PacketID: packetID}
	if err := header.Write(buffer); err != nil {
		return nil, err
	}
	if _, err := buffer.Write(payload); err != nil {
		return nil, err
	}
	return decodeNexusPacket(buffer.Bytes())
}

func encodeNexusPacket(pk nexuspacket.Packet) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	header := nexuspacket.Header{PacketID: pk.ID()}
	if err := header.Write(buffer); err != nil {
		return nil, err
	}
	writer := nexusprotocol.NewWriter(buffer, 0)
	pk.Marshal(writer)
	return buffer.Bytes(), nil
}

func decodeNexusPacket(raw []byte) (result nexuspacket.Packet, err error) {
	buffer := bytes.NewBuffer(raw)
	header := nexuspacket.Header{}
	if err := header.Read(buffer); err != nil {
		return nil, err
	}
	factory, ok := nexuspacket.NewPool()[header.PacketID]
	if !ok {
		return &nexuspacket.Unknown{PacketID: header.PacketID, Payload: append([]byte(nil), buffer.Bytes()...)}, nil
	}
	pk := factory()
	reader := nexusprotocol.NewReader(buffer, 0, false)
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("decode Nexus packet %d: %v", header.PacketID, recovered)
		}
	}()
	pk.Marshal(reader)
	return pk, nil
}

func decodeStarPacket(raw []byte) (result starpacket.Packet, err error) {
	buffer := bytes.NewBuffer(raw)
	header := starpacket.Header{}
	if err := header.Read(buffer); err != nil {
		return nil, err
	}
	factory, ok := starpacket.ListAllPackets()[header.PacketID]
	if !ok {
		return &starpacket.Unknown{PacketID: header.PacketID, Payload: append([]byte(nil), buffer.Bytes()...)}, nil
	}
	pk := factory()
	reader := starprotocol.NewReader(buffer, 0, false)
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("decode RaaBel packet %d: %v", header.PacketID, recovered)
		}
	}()
	pk.Marshal(reader)
	return pk, nil
}

func setPrivateField(target any, fieldName string, value any) {
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

type packetProxyConn struct {
	ctx   context.Context
	close context.CancelFunc
	write func([]byte) (int, error)
}

func newPacketProxyConn() *packetProxyConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &packetProxyConn{ctx: ctx, close: cancel}
}

func (p *packetProxyConn) Read(_ []byte) (int, error) {
	<-p.ctx.Done()
	return 0, io.EOF
}

func (p *packetProxyConn) Write(b []byte) (int, error) {
	if p.write != nil {
		n, err := p.write(b)
		if n == 0 && err == nil {
			return len(b), nil
		}
		return n, err
	}
	return len(b), nil
}

func (p *packetProxyConn) Close() error {
	p.close()
	return nil
}

func (p *packetProxyConn) LocalAddr() net.Addr              { return proxyAddr("nexus-local") }
func (p *packetProxyConn) RemoteAddr() net.Addr             { return proxyAddr("starshuttler-proxy") }
func (p *packetProxyConn) SetDeadline(time.Time) error      { return nil }
func (p *packetProxyConn) SetReadDeadline(time.Time) error  { return nil }
func (p *packetProxyConn) SetWriteDeadline(time.Time) error { return nil }
func (p *packetProxyConn) Context() context.Context         { return p.ctx }

type proxyAddr string

func (p proxyAddr) Network() string { return "starshuttler-runtime" }
func (p proxyAddr) String() string  { return string(p) }

var _ = unsafe.Pointer(nil)
