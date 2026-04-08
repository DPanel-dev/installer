package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// parseYes 在终端显示确认提示，返回用户是否确认
func parseYes(msg string) bool {
	fmt.Printf("%s [y/N]: ", msg)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
