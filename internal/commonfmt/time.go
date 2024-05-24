package commonfmt

import (
	"fmt"
	"strconv"
	"time"
)

func FmtInoxDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	v := time.Duration(d)
	b := make([]byte, 0, 32)

	for v != 0 {
		switch {
		case v >= time.Hour:
			b = strconv.AppendUint(b, uint64(v/time.Hour), 10)
			b = append(b, 'h')
			v %= time.Hour
		case v >= time.Minute:
			b = strconv.AppendUint(b, uint64(v/time.Minute), 10)
			b = append(b, "mn"...)
			v %= time.Minute
		case v >= time.Second:
			b = strconv.AppendUint(b, uint64(v/time.Second), 10)
			b = append(b, 's')
			v %= time.Second
		case v >= time.Millisecond:
			b = strconv.AppendUint(b, uint64(v/time.Millisecond), 10)
			b = append(b, "ms"...)
			v %= time.Millisecond
		case v >= time.Microsecond:
			b = strconv.AppendUint(b, uint64(v/time.Microsecond), 10)
			b = append(b, "us"...)
			v %= time.Microsecond
		default:
			b = strconv.AppendUint(b, uint64(v), 10)
			b = append(b, "ns"...)
			v = 0
		}
	}

	return string(b)
}

func FmtInoxYear(d time.Time) string {
	// TODO: change
	t := d.UTC()

	return fmt.Sprintf("%dy-%s",
		t.Year(), t.Location().String())
}

func FmtInoxDate(d time.Time) string {
	// TODO: change
	t := d.UTC()

	return fmt.Sprintf("%dy-%dmt-%dd-%s",
		t.Year(), t.Month(), t.Day(), t.Location().String())
}

func FmtInoxDateTime(d time.Time) string {
	// TODO: change
	t := d.UTC()
	ns := t.Nanosecond()
	ms := ns / 1_000_000
	us := (ns % 1_000_000) / 1000

	return fmt.Sprintf("%dy-%dmt-%dd-%dh-%dm-%ds-%dms-%dus-%s",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), ms, us, t.Location().String())
}
