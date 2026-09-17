package ui

// Cursor and viewport arithmetic shared by every list in the package (the
// palette, the queue editor, the focus picker, the init wizard), so they all
// clamp and scroll the same way.

// moveCursor returns cursor moved by delta and clamped to a list of n rows; an
// empty list keeps the cursor at 0.
func moveCursor(cursor, delta, n int) int {
	if n <= 0 {
		return 0
	}
	cursor += delta
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// scrollToCursor returns the viewport offset that keeps cursor visible in a
// window of rows lines, moving the window only as far as needed.
func scrollToCursor(cursor, offset, rows int) int {
	if rows < 1 {
		rows = 1
	}
	if cursor >= offset+rows {
		offset = cursor - rows + 1
	}
	if cursor < offset {
		offset = cursor
	}
	if offset < 0 {
		return 0
	}
	return offset
}

// visibleWindow returns the [start, end) range of n rows drawn from offset in a
// window of rows lines.
func visibleWindow(offset, rows, n int) (int, int) {
	end := offset + rows
	if end > n {
		end = n
	}
	if offset > end {
		offset = end
	}
	return offset, end
}

// padLines appends blank lines until there are height of them, so a short list
// still fills its area.
func padLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}
