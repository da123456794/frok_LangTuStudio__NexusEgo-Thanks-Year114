package midi

import (
	"fmt"
	"math"
	"sort"
)

const (
	defaultTicksPerSecond = 20.0
	defaultSpeed          = 1.0
	defaultSelector       = "@a"
	defaultMelodic        = "note.harp"
	defaultPercussion     = "note.bd"
	defaultSquareSize     = int32(8)
)

type ConvertOptions struct {
	TicksPerSecond    float64
	Speed             float64
	MasterVolume      float64
	Selector          string
	DefaultMelodic    string
	DefaultPercussion string
	PitchOffset       float64
	WrapPitch         bool
	SquareSize        int32
}

type Timeline struct {
	CommandsByTick map[int][]string
	LastTick       int
}

func DefaultOptions() ConvertOptions {
	return ConvertOptions{
		TicksPerSecond:    defaultTicksPerSecond,
		Speed:             defaultSpeed,
		MasterVolume:      1.0,
		Selector:          defaultSelector,
		DefaultMelodic:    defaultMelodic,
		DefaultPercussion: defaultPercussion,
		SquareSize:        defaultSquareSize,
	}
}

func BuildTimeline(song *Song, opts ConvertOptions) (*Timeline, error) {
	if song == nil {
		return nil, fmt.Errorf("song is nil")
	}
	if opts.TicksPerSecond <= 0 {
		opts.TicksPerSecond = defaultTicksPerSecond
	}
	if opts.Speed <= 0 {
		opts.Speed = defaultSpeed
	}
	if opts.MasterVolume <= 0 {
		opts.MasterVolume = 1.0
	}
	if opts.Selector == "" {
		opts.Selector = defaultSelector
	}
	if opts.DefaultMelodic == "" {
		opts.DefaultMelodic = defaultMelodic
	}
	if opts.DefaultPercussion == "" {
		opts.DefaultPercussion = defaultPercussion
	}
	if opts.SquareSize <= 0 {
		opts.SquareSize = defaultSquareSize
	}

	notes := append([]NoteEvent{}, song.Notes...)
	sort.Slice(notes, func(i, j int) bool { return notes[i].Tick < notes[j].Tick })
	segments := buildTempoSegments(song.Tempos, song.Division)
	if len(segments) == 0 {
		segments = []tempoSegment{{startTick: 0, startSec: 0, secondsPerTick: float64(defaultTempoMPQN) / 1e6 / float64(song.Division)}}
	}

	cmdBuckets := make(map[int][]string)
	lastTick := 0
	segIndex := 0
	prevSeconds := 0.0
	currentTick := 0
	hasPrev := false

	for _, note := range notes {
		for segIndex+1 < len(segments) && note.Tick >= segments[segIndex+1].startTick {
			segIndex++
		}
		seg := segments[segIndex]
		deltaTicks := float64(note.Tick - seg.startTick)
		seconds := seg.startSec + deltaTicks*seg.secondsPerTick
		seconds /= opts.Speed

		deltaSeconds := seconds
		if hasPrev {
			deltaSeconds = seconds - prevSeconds
		}
		tickDelta := int(math.Round(deltaSeconds * opts.TicksPerSecond))
		if tickDelta < 0 {
			tickDelta = 0
		}
		currentTick += tickDelta
		mcTick := currentTick
		prevSeconds = seconds
		hasPrev = true

		if mcTick < 0 {
			continue
		}

		volume := opts.MasterVolume * float64(note.Velocity) * 0.01
		if volume < 0.01 {
			volume = 0.01
		}
		sound, pitch := soundForEvent(note, opts)
		command := fmt.Sprintf(
			"execute as %s at @s run playsound %s @s ^^^8 %.5f %.5f %.5f",
			opts.Selector, sound, volume, pitch, volume,
		)
		cmdBuckets[mcTick] = append(cmdBuckets[mcTick], command)
		if mcTick > lastTick {
			lastTick = mcTick
		}
	}

	return &Timeline{
		CommandsByTick: cmdBuckets,
		LastTick:       lastTick,
	}, nil
}

type tempoSegment struct {
	startTick      uint64
	startSec       float64
	secondsPerTick float64
}

func buildTempoSegments(tempos []TempoChange, division uint16) []tempoSegment {
	if division == 0 || len(tempos) == 0 {
		return nil
	}
	sort.Slice(tempos, func(i, j int) bool { return tempos[i].Tick < tempos[j].Tick })
	segments := make([]tempoSegment, 0, len(tempos))
	for i, tempo := range tempos {
		if i == 0 {
			segments = append(segments, tempoSegment{
				startTick:      tempo.Tick,
				startSec:       0,
				secondsPerTick: float64(tempo.MicroPerQuart) / 1e6 / float64(division),
			})
			continue
		}
		prev := segments[len(segments)-1]
		deltaTicks := float64(tempo.Tick - prev.startTick)
		startSec := prev.startSec + deltaTicks*prev.secondsPerTick
		segments = append(segments, tempoSegment{
			startTick:      tempo.Tick,
			startSec:       startSec,
			secondsPerTick: float64(tempo.MicroPerQuart) / 1e6 / float64(division),
		})
	}
	return segments
}

func soundForEvent(note NoteEvent, opts ConvertOptions) (string, float64) {
	if note.Channel == 9 {
		return percussionSound(note.Note, opts.DefaultPercussion), 1.0
	}
	return melodicSound(note.Program, opts.DefaultMelodic), noteToPitch(note.Note, opts.PitchOffset, opts.WrapPitch)
}

func melodicSound(program uint8, defaultSound string) string {
	switch {
	case program == 105:
		return "note.banjo"
	case program >= 32 && program <= 39:
		return "note.bass"
	case program >= 115 && program <= 118:
		return "note.basedrum"
	case program == 9:
		return "note.bell"
	case program == 80 || program == 81:
		return "note.bit"
	case program == 112:
		return "note.cow_bell"
	case (program >= 72 && program <= 79) || (program >= 41 && program <= 44):
		return "note.flute"
	case program >= 24 && program <= 31:
		return "note.guitar"
	case program == 14:
		return "note.chime"
	case program >= 8 && program <= 15 && program != 14:
		return "note.iron_xylophone"
	case program == 2:
		return "note.pling"
	default:
		return defaultSound
	}
}

func percussionSound(note uint8, defaultSound string) string {
	switch note {
	case 55:
		return "note.cow_bell"
	case 41, 43, 45:
		return "note.hat"
	case 36, 37, 39:
		return "note.snare"
	default:
		return defaultSound
	}
}

func noteToPitch(note uint8, offset float64, wrap bool) float64 {
	semitones := float64(int(note)) + offset - 66
	pitch := math.Pow(2, semitones/12.0)
	if wrap {
		for pitch < 0.5 {
			pitch *= 2
		}
		for pitch > 2.0 {
			pitch /= 2
		}
	}
	return pitch
}

func SongDurationSeconds(song *Song) float64 {
	if song == nil || song.Division == 0 || len(song.Notes) == 0 {
		return 0
	}
	lastTick := song.Notes[0].Tick
	for _, note := range song.Notes[1:] {
		if note.Tick > lastTick {
			lastTick = note.Tick
		}
	}
	segments := buildTempoSegments(song.Tempos, song.Division)
	if len(segments) == 0 {
		segments = []tempoSegment{{startTick: 0, startSec: 0, secondsPerTick: float64(defaultTempoMPQN) / 1e6 / float64(song.Division)}}
	}
	segIndex := 0
	for segIndex+1 < len(segments) && lastTick >= segments[segIndex+1].startTick {
		segIndex++
	}
	seg := segments[segIndex]
	deltaTicks := float64(lastTick - seg.startTick)
	return seg.startSec + deltaTicks*seg.secondsPerTick
}
