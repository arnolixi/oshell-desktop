package sftp

import (
	"regexp"
	"strconv"
	"strings"
)

var unixOwnerLine = regexp.MustCompile(`^([bcdlps-][rwxstST-]{9})[+@.]?\s+\d+\s+(\S+)\s+(\S+)\s+(\d+)\s+[A-Za-z]{3}\s+\d{1,2}\s+(?:\d{2}:\d{2}(?::\d{2})?|\d{4})\s+(.*)$`)

// Long names are hints only; never use them to replace authoritative attributes.
func retainOwnerNames(stat *FileStat, name, line string) {
	m := unixOwnerLine.FindStringSubmatch(line)
	if m == nil {
		return
	}
	size, err := strconv.ParseUint(m[4], 10, 64)
	if err != nil || size != stat.Size {
		return
	}
	listed := m[5]
	if stat.Mode&0170000 == 0120000 {
		listed = strings.SplitN(listed, " -> ", 2)[0]
	}
	if listed != name {
		return
	}
	var mode uint32
	for g := 0; g < 3; g++ {
		pos := 1 + g*3
		shift := uint((2 - g) * 3)
		if m[1][pos] == 'r' {
			mode |= 4 << shift
		} else if m[1][pos] != '-' {
			return
		}
		if m[1][pos+1] == 'w' {
			mode |= 2 << shift
		} else if m[1][pos+1] != '-' {
			return
		}
		ch := m[1][pos+2]
		lo, hi := byte('s'), byte('S')
		if g == 2 {
			lo, hi = 't', 'T'
		}
		if ch == 'x' || ch == lo {
			mode |= 1 << shift
		}
		if ch == lo || ch == hi {
			mode |= []uint32{04000, 02000, 01000}[g]
		} else if ch != 'x' && ch != '-' {
			return
		}
	}
	kinds := map[byte]uint32{'-': 0100000, 'd': 0040000, 'l': 0120000, 'b': 0060000, 'c': 0020000, 'p': 0010000, 's': 0140000}
	if stat.Mode&0177777 != kinds[m[1][0]]|mode {
		return
	}
	stat.Extended = append(stat.Extended, StatExtended{"unix.ownername", m[2]}, StatExtended{"unix.groupname", m[3]})
}
