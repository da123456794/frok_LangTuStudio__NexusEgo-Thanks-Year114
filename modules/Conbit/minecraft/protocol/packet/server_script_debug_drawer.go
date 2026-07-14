package packet

import (
	"image/color"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	ScriptDebugShapeLine = iota
	ScriptDebugShapeBox
	ScriptDebugShapeSphere
	ScriptDebugShapeCircle
	ScriptDebugShapeText
	ScriptDebugShapeArrow
)

// DebugDrawerShape defines a single debug shape to be rendered on the client.
type DebugDrawerShape struct {
	NetworkID       uint64
	Type            protocol.Optional[uint8]
	Location        protocol.Optional[mgl32.Vec3]
	Scale           protocol.Optional[float32]
	Rotation        protocol.Optional[mgl32.Vec3]
	TotalTimeLeft   protocol.Optional[float32]
	Colour          protocol.Optional[color.RGBA]
	Text            protocol.Optional[string]
	BoxBound        protocol.Optional[mgl32.Vec3]
	LineEndLocation protocol.Optional[mgl32.Vec3]
	ArrowHeadLength protocol.Optional[float32]
	ArrowHeadRadius protocol.Optional[float32]
	Segments        protocol.Optional[byte]
}

func (x *DebugDrawerShape) Marshal(io protocol.IO) {
	io.Varuint64(&x.NetworkID)
	protocol.OptionalFunc(io, &x.Type, io.Uint8)
	protocol.OptionalFunc(io, &x.Location, io.Vec3)
	protocol.OptionalFunc(io, &x.Scale, io.Float32)
	protocol.OptionalFunc(io, &x.Rotation, io.Vec3)
	protocol.OptionalFunc(io, &x.TotalTimeLeft, io.Float32)
	protocol.OptionalFunc(io, &x.Colour, io.BEARGB)
	protocol.OptionalFunc(io, &x.Text, io.String)
	protocol.OptionalFunc(io, &x.BoxBound, io.Vec3)
	protocol.OptionalFunc(io, &x.LineEndLocation, io.Vec3)
	protocol.OptionalFunc(io, &x.ArrowHeadLength, io.Float32)
	protocol.OptionalFunc(io, &x.ArrowHeadRadius, io.Float32)
	protocol.OptionalFunc(io, &x.Segments, io.Uint8)
}

// ServerScriptDebugDrawer instructs the client to render one or more debug shapes.
type ServerScriptDebugDrawer struct {
	Shapes []DebugDrawerShape
}

// ID ...
func (pk *ServerScriptDebugDrawer) ID() uint32 {
	return IDServerScriptDebugDrawer
}

func (pk *ServerScriptDebugDrawer) Marshal(io protocol.IO) {
	protocol.Slice(io, &pk.Shapes)
}
