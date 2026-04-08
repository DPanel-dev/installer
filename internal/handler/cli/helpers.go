package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// parseYes 在终端显示确认提示，返回用户是否确认
// 格式与 slog 输出对齐：HH:MM:SS WARN msg [y/N]:
func parseYes(msg string) bool {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("%s WARN %s [y/N]: ", ts, msg)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
