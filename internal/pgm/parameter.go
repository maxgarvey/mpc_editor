package pgm

import "fmt"

// ParamType defines how a parameter value is read/written from a buffer.
type ParamType int

const (
	TypeInt    ParamType = iota // byte value with min/max range
	TypeOffInt                  // byte value where 0 means "Off"
	TypeString                  // 16-char null-terminated string
	TypeEnum                    // byte index into a list of string values
	TypeTuning                  // little-endian int16, stored as value*100
	TypeRange                   // pair of bytes (low, high)
)

// Parameter defines a named, typed field at a specific byte offset within an element.
type Parameter struct {
	Label      string
	Offset     int
	Type       ParamType
	Min        int
	Max        int
	EnumValues []string // only for TypeEnum
}

// IntParam creates an integer parameter with a range.
func IntParam(label string, offset, min, max int) Parameter {
	return Parameter{Label: label, Offset: offset, Type: TypeInt, Min: min, Max: max}
}

// OffIntParam creates an integer parameter where 0 means "Off".
func OffIntParam(label string, offset, min, max int) Parameter {
	return Parameter{Label: label, Offset: offset, Type: TypeOffInt, Min: min, Max: max}
}

// StringParam creates a 16-char string parameter.
func StringParam(label string, offset int) Parameter {
	return Parameter{Label: label, Offset: offset, Type: TypeString}
}

// EnumParam creates an enumerated parameter with named values.
func EnumParam(label string, offset int, values []string) Parameter {
	return Parameter{Label: label, Offset: offset, Type: TypeEnum, Min: 0, Max: len(values) - 1, EnumValues: values}
}

// TuningParam creates a tuning parameter (int16, value/100 = semitones).
func TuningParam(label string, offset int, min, max int) Parameter {
	return Parameter{Label: label, Offset: offset, Type: TypeTuning, Min: min, Max: max}
}

// RangeParam creates a range parameter (pair of bytes).
func RangeParam(label string, offset, min, max int) Parameter {
	return Parameter{Label: label, Offset: offset, Type: TypeRange, Min: min, Max: max}
}

// Get reads the parameter value from the buffer at the given base offset.
// Returns: int for Int/OffInt/Enum, float64 for Tuning, string for String, Range for Range.
func (p Parameter) Get(buf *Buffer, base int) interface{} {
	off := base + p.Offset
	switch p.Type {
	case TypeInt, TypeOffInt, TypeEnum:
		return int(buf.GetByte(off))
	case TypeString:
		return buf.GetString(off)
	case TypeTuning:
		return float64(buf.GetShort(off)) / 100.0
	case TypeRange:
		return buf.GetRange(off)
	default:
		return nil
	}
}

// Set writes the parameter value to the buffer at the given base offset.
func (p Parameter) Set(buf *Buffer, base int, value interface{}) error {
	off := base + p.Offset
	switch p.Type {
	case TypeInt, TypeOffInt, TypeEnum:
		v, ok := toInt(value)
		if !ok {
			return fmt.Errorf("parameter %s: expected int, got %T", p.Label, value)
		}
		if v < p.Min || v > p.Max {
			return fmt.Errorf("parameter %s: value %d out of range [%d, %d]", p.Label, v, p.Min, p.Max)
		}
		buf.SetByte(off, byte(v))
	case TypeString:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("parameter %s: expected string, got %T", p.Label, value)
		}
		return buf.SetString(off, s)
	case TypeTuning:
		f, ok := toFloat(value)
		if !ok {
			return fmt.Errorf("parameter %s: expected number, got %T", p.Label, value)
		}
		raw := int16(f * 100.0)
		buf.SetShort(off, raw)
	case TypeRange:
		r, ok := value.(Range)
		if !ok {
			return fmt.Errorf("parameter %s: expected Range, got %T", p.Label, value)
		}
		buf.SetRange(off, r)
	}
	return nil
}

// Validate checks if a value is valid for this parameter.
func (p Parameter) Validate(value interface{}) bool {
	switch p.Type {
	case TypeInt, TypeOffInt, TypeEnum:
		v, ok := toInt(value)
		if !ok {
			return false
		}
		return v >= p.Min && v <= p.Max
	case TypeString:
		s, ok := value.(string)
		if !ok {
			return false
		}
		return len(s) <= 16
	case TypeTuning:
		f, ok := toFloat(value)
		return ok && f >= float64(p.Min) && f <= float64(p.Max)
	case TypeRange:
		_, ok := value.(Range)
		return ok
	}
	return false
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case byte:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
