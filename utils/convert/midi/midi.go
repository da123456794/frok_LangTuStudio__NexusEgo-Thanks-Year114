package midi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	eventNoteOn        = 0x9
	eventProgramChange = 0xC
	eventMeta          = 0xFF
	eventSysEx         = 0xF0
	eventSysExEscape   = 0xF7
	metaSetTempo       = 0x51
	defaultTempoMPQN   = 500000
)

type NoteEvent struct {
	Tick     uint64
	Channel  uint8
	Note     uint8
	Velocity uint8
	Program  uint8
}

type TempoChange struct {
	Tick          uint64
	MicroPerQuart int
}

type Song struct {
	Division uint16
	Notes    []NoteEvent
	Tempos   []TempoChange
}

type rawEvent struct {
	tick      uint64
	order     int
	kind      byte
	channel   uint8
	note      uint8
	velocity  uint8
	program   uint8
	tempoMPQN int
}

func ParseFile(path string) (*Song, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read midi: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Song, error) {
	r := bytes.NewReader(data)
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if string(header) != "MThd" {
		return nil, fmt.Errorf("invalid header: %q", string(header))
	}

	var headerLen uint32
	if err := binary.Read(r, binary.BigEndian, &headerLen); err != nil {
		return nil, fmt.Errorf("read header length: %w", err)
	}
	if headerLen < 6 {
		return nil, fmt.Errorf("invalid header length: %d", headerLen)
	}

	var format uint16
	var tracks uint16
	var division uint16
	if err := binary.Read(r, binary.BigEndian, &format); err != nil {
		return nil, fmt.Errorf("read format: %w", err)
	}
	if err := binary.Read(r, binary.BigEndian, &tracks); err != nil {
		return nil, fmt.Errorf("read track count: %w", err)
	}
	if err := binary.Read(r, binary.BigEndian, &division); err != nil {
		return nil, fmt.Errorf("read division: %w", err)
	}
	if division&0x8000 != 0 {
		return nil, fmt.Errorf("smpte timing not supported")
	}
	if headerLen > 6 {
		if _, err := r.Seek(int64(headerLen-6), io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("skip header: %w", err)
		}
	}

	rawEvents := make([]rawEvent, 0, 1024)
	order := 0
	for i := 0; i < int(tracks); i++ {
		if _, err := io.ReadFull(r, header); err != nil {
			return nil, fmt.Errorf("read track header: %w", err)
		}
		if string(header) != "MTrk" {
			return nil, fmt.Errorf("invalid track header: %q", string(header))
		}

		var trackLen uint32
		if err := binary.Read(r, binary.BigEndian, &trackLen); err != nil {
			return nil, fmt.Errorf("read track length: %w", err)
		}
		trackData := make([]byte, trackLen)
		if _, err := io.ReadFull(r, trackData); err != nil {
			return nil, fmt.Errorf("read track data: %w", err)
		}
		trackEvents, nextOrder, err := parseTrack(trackData, order)
		if err != nil {
			return nil, err
		}
		order = nextOrder
		rawEvents = append(rawEvents, trackEvents...)
	}

	sort.Slice(rawEvents, func(i, j int) bool {
		if rawEvents[i].tick == rawEvents[j].tick {
			return rawEvents[i].order < rawEvents[j].order
		}
		return rawEvents[i].tick < rawEvents[j].tick
	})

	tempos := []TempoChange{{Tick: 0, MicroPerQuart: defaultTempoMPQN}}
	for _, ev := range rawEvents {
		if ev.kind == metaSetTempo {
			tempos = append(tempos, TempoChange{Tick: ev.tick, MicroPerQuart: ev.tempoMPQN})
		}
	}
	sort.Slice(tempos, func(i, j int) bool { return tempos[i].Tick < tempos[j].Tick })
	tempos = dedupeTempos(tempos)

	var programByChannel [16]uint8
	notes := make([]NoteEvent, 0, len(rawEvents)/2)
	for _, ev := range rawEvents {
		switch ev.kind {
		case eventProgramChange:
			programByChannel[ev.channel] = ev.program
		case eventNoteOn:
			if ev.velocity == 0 {
				continue
			}
			notes = append(notes, NoteEvent{
				Tick:     ev.tick,
				Channel:  ev.channel,
				Note:     ev.note,
				Velocity: ev.velocity,
				Program:  programByChannel[ev.channel],
			})
		}
	}

	return &Song{
		Division: division,
		Notes:    notes,
		Tempos:   tempos,
	}, nil
}

func dedupeTempos(tempos []TempoChange) []TempoChange {
	if len(tempos) == 0 {
		return tempos
	}
	out := []TempoChange{tempos[0]}
	for i := 1; i < len(tempos); i++ {
		prev := out[len(out)-1]
		if tempos[i].Tick == prev.Tick {
			out[len(out)-1] = tempos[i]
			continue
		}
		out = append(out, tempos[i])
	}
	return out
}

func parseTrack(data []byte, order int) ([]rawEvent, int, error) {
	r := bytes.NewReader(data)
	var events []rawEvent
	var tick uint64
	var runningStatus byte
	for r.Len() > 0 {
		delta, err := readVLQ(r)
		if err != nil {
			return nil, order, fmt.Errorf("read delta: %w", err)
		}
		tick += uint64(delta)

		status, err := r.ReadByte()
		if err != nil {
			return nil, order, fmt.Errorf("read status: %w", err)
		}
		if status < 0x80 {
			if runningStatus == 0 {
				return nil, order, fmt.Errorf("running status without previous status")
			}
			if err := r.UnreadByte(); err != nil {
				return nil, order, fmt.Errorf("unread data byte: %w", err)
			}
			status = runningStatus
		} else {
			runningStatus = status
		}

		switch {
		case status == eventMeta:
			metaType, err := r.ReadByte()
			if err != nil {
				return nil, order, fmt.Errorf("read meta type: %w", err)
			}
			length, err := readVLQ(r)
			if err != nil {
				return nil, order, fmt.Errorf("read meta length: %w", err)
			}
			if metaType == metaSetTempo && length == 3 {
				buf := make([]byte, 3)
				if _, err := io.ReadFull(r, buf); err != nil {
					return nil, order, fmt.Errorf("read tempo: %w", err)
				}
				mpqn := int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
				events = append(events, rawEvent{
					tick:      tick,
					order:     order,
					kind:      metaSetTempo,
					tempoMPQN: mpqn,
				})
				order++
				continue
			}
			if _, err := r.Seek(int64(length), io.SeekCurrent); err != nil {
				return nil, order, fmt.Errorf("skip meta: %w", err)
			}

		case status == eventSysEx || status == eventSysExEscape:
			length, err := readVLQ(r)
			if err != nil {
				return nil, order, fmt.Errorf("read sysex length: %w", err)
			}
			if _, err := r.Seek(int64(length), io.SeekCurrent); err != nil {
				return nil, order, fmt.Errorf("skip sysex: %w", err)
			}

		default:
			eventType := status >> 4
			channel := status & 0x0F
			switch eventType {
			case 0x8:
				if _, err := r.ReadByte(); err != nil {
					return nil, order, fmt.Errorf("read note off: %w", err)
				}
				if _, err := r.ReadByte(); err != nil {
					return nil, order, fmt.Errorf("read note off velocity: %w", err)
				}
			case eventNoteOn:
				note, err := r.ReadByte()
				if err != nil {
					return nil, order, fmt.Errorf("read note on: %w", err)
				}
				vel, err := r.ReadByte()
				if err != nil {
					return nil, order, fmt.Errorf("read note velocity: %w", err)
				}
				events = append(events, rawEvent{
					tick:     tick,
					order:    order,
					kind:     eventNoteOn,
					channel:  channel,
					note:     note,
					velocity: vel,
				})
				order++
			case eventProgramChange:
				prog, err := r.ReadByte()
				if err != nil {
					return nil, order, fmt.Errorf("read program change: %w", err)
				}
				events = append(events, rawEvent{
					tick:    tick,
					order:   order,
					kind:    eventProgramChange,
					channel: channel,
					program: prog,
				})
				order++
			case 0xA, 0xB, 0xE:
				if _, err := r.ReadByte(); err != nil {
					return nil, order, fmt.Errorf("read midi data: %w", err)
				}
				if _, err := r.ReadByte(); err != nil {
					return nil, order, fmt.Errorf("read midi data: %w", err)
				}
			case 0xD:
				if _, err := r.ReadByte(); err != nil {
					return nil, order, fmt.Errorf("read channel pressure: %w", err)
				}
			default:
				return nil, order, fmt.Errorf("unknown midi status: 0x%X", status)
			}
		}
	}

	return events, order, nil
}

func readVLQ(r *bytes.Reader) (uint32, error) {
	var value uint32
	for i := 0; i < 4; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value = (value << 7) | uint32(b&0x7F)
		if b&0x80 == 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("invalid vlq")
}
