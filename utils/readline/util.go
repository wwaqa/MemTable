package readline

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func IsOrdinaryInput(input byte) bool {
	return input >= 32 && input <= 126
}

// TwinkleScreen 闪烁一次屏幕
func TwinkleScreen() {
	x, y := ReadCursor()
	// 保存屏幕内容，然后清屏
	FlushString(fmt.Sprintf("\033[?47h\033[2J\033[%d;%dH", y, x))
	time.Sleep(5 * time.Millisecond)
	// 等待一段时间后恢复屏幕内容
	FlushString("\033[?47l")
}

func ClearLine(y int) {
	ox, oy := ReadCursor()
	MoveCursorTo(0, y)
	FlushString("\033[K")
	MoveCursorTo(ox, oy)
}

// ReadCursor 读取当前光标的位置
func ReadCursor() (x, y int) {
	FlushString("\033[6n")
	_, _ = fmt.Scanf("\033[%d;%dR", &y, &x)
	return x, y
}

// Flush 输出到屏幕
func Flush(content []byte) {
	_, _ = os.Stdout.Write(content)
}

// FlushString 输出到屏幕
func FlushString(content string) {
	_, _ = os.Stdout.WriteString(content)
}

func FlushStringWithUnderline(content string) {
	_, _ = os.Stdout.WriteString("\033[4m" + content + "\033[0m")
}

// MoveCursorTo 将光标移动到目标位置
func MoveCursorTo(dstX, dstY int) {
	_, _ = os.Stdout.WriteString(fmt.Sprintf("\033[%d;%dH", dstY, dstX))
}

// MoveCursor 将光标移动指定的偏移量
func MoveCursor(x, y int) {

	if x < 0 {
		_, _ = os.Stdout.WriteString(fmt.Sprintf("\033[%dD", 0-x))
	} else if x > 0 {
		_, _ = os.Stdout.WriteString(fmt.Sprintf("\033[%dC", x))
	}

	if y < 0 {
		_, _ = os.Stdout.WriteString(fmt.Sprintf("\033[%dA", -y))
	} else if y > 0 {
		_, _ = os.Stdout.WriteString(fmt.Sprintf("\033[%dB", y))
	}
}

func DisableTerminal() *Termios {

	newState, _ := getTermios(int(os.Stdin.Fd()))
	oldState := *newState
	// This attempts to replicate the behaviour documented for cfmakeraw in
	// the termios(3) manpage.

	newState.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR |
		syscall.IGNCR | syscall.ICRNL | syscall.IXON

	//newState.Oflag &^= syscall.OPOST

	newState.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN

	newState.Cflag &^= syscall.CSIZE | syscall.PARENB
	newState.Cflag |= syscall.CS8

	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0

	_ = setTermios(int(os.Stdin.Fd()), newState)

	return &oldState
}

// SplitRepeatableSeg 会将 s 按照 seg 来进行切割，忽略 "" 之间的 seg
func SplitRepeatableSeg(s []byte, seg byte) [][]byte {
	var splits [][]byte
	i, j := 0, 0
	for ; j < len(s); j++ {
		if s[j] == seg {
			if j > i {
				splits = append(splits, s[i:j])
				i = j + 1
			} else {
				i++
			}
		} else if s[j] == '"' && (j == 0 || s[j-1] == seg) {
			k := j + 1
			for ; k < len(s); k++ {
				if s[k] == '"' && s[k-1] != '\\' && (k == len(s)-1 || s[k+1] == seg) {
					splits = append(splits, s[j+1:k])
					i, j = k+1, k
					break
				}
			}
		}
	}
	if i < len(s) && j > i {
		splits = append(splits, s[i:j])
	}
	return splits
}
