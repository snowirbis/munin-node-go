package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	slog "github.com/OloloevReal/go-simple-log"
)

const (
	lineMax        = 2048
	version        = "1.0.6-go"
	nodeConfigPath = "node.conf"
)

type NodeConfig struct {
	HostName     string
	AllowedIPs   []string
	Host         string
	Port         string
	PluginFolder string
	PluginConfig string
}

var nodeConf = NodeConfig{}

func readNodeConfig(configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("could not open configuration file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "host_name":
			nodeConf.HostName = value
		case "allow":
			nodeConf.AllowedIPs = append(nodeConf.AllowedIPs, value)
		case "host":
			if value == "*" {
				nodeConf.Host = ""
			} else {
				nodeConf.Host = value
			}
		case "port":
			nodeConf.Port = value
		case "plugins":
			nodeConf.PluginFolder = value
		case "plugins_config":
			nodeConf.PluginConfig = value
		}

	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading configuration file: %w", err)
	}

	return nil
}

func isAllowedIP(clientIP string, allowedPatterns []string) bool {
	for _, pattern := range allowedPatterns {
		match, err := regexp.MatchString(pattern, clientIP)
		if err != nil {
			fmt.Printf("Error in IP permission template: %v\n", err)
			continue
		}
		if match {
			return true
		}
	}
	return false
}

func listPlugins() string {
	files, err := ioutil.ReadDir(nodeConf.PluginFolder)
	if err != nil {
		slog.Printf("failed to read directory %s: %w", nodeConf.PluginFolder, err)
		return ""
	}

	var plugins []string
	for _, file := range files {
		if !file.IsDir() {
			plugins = append(plugins, file.Name())
		}
	}

	return strings.Join(plugins, " ") + "\n"
}

func loadPluginConfig(plugin string) error {
	absPluginConf, err := filepath.Abs(nodeConf.PluginConfig)
	if err != nil {
		return fmt.Errorf("failed to get absolute path to plugin config: %w", err)
	}

	file, err := os.Open(absPluginConf)
	if err != nil {
		return fmt.Errorf("unable to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentSection string

	possibleSections := generatePossibleSections(plugin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := line[1 : len(line)-1]

			for _, sec := range possibleSections {
				if section == sec {
					currentSection = section
					break
				}
			}
			continue
		}

		if currentSection != "" && strings.HasPrefix(line, "env.") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid string format: %s", line)
			}

			key := strings.TrimPrefix(parts[0], "env.")
			value := strings.TrimSpace(parts[1])

			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("failed to set environment variable: %w", err)
			}

			slog.Printf("env variable %s with value %s set for plugin %s\n", key, value, plugin)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("file read error: %w", err)
	}

	slog.Println("env variables successfully set for plugin:", plugin)

	return nil
}

func generatePossibleSections(plugin string) []string {
	var sections []string

	// Load global variables from [*] section
	sections = append(sections, "*")
	
	parts := strings.Split(plugin, "_")

	for i := len(parts); i > 0; i-- {
		sections = append(sections, strings.Join(parts[:i], "_")+"_*")
	}

	return sections
}

func validatePluginPath(pluginPath string) error {

	absPluginPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return fmt.Errorf("не вдалося отримати абсолютний шлях до плагіна: %w", err)
	}

	absAllowedDir, err := filepath.Abs(nodeConf.PluginFolder)
	if err != nil {
		return fmt.Errorf("failed to get absolute path to allowed folder: %w", err)
	}

	if !strings.HasPrefix(absPluginPath, absAllowedDir) {
		return fmt.Errorf("plugin is outside the allowed folder: %s", absAllowedDir)
	}

	fileInfo, err := os.Lstat(absPluginPath)
	if err != nil {
		return fmt.Errorf("failed to get plugin information: %w", err)
	}

	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("plugin is a symbolic link: %s", absPluginPath)
	}

	return nil
}

func executePlugin(plugin string, option string) (string, error) {

	pluginPath := filepath.Join(nodeConf.PluginFolder, plugin)

	err := validatePluginPath(pluginPath)
	if err != nil {
		return "", err
	}

	err = loadPluginConfig(plugin)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(pluginPath, option)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("plugin failed to execute: %w", err)
	}

	return string(output), nil
}

func startNode() error {
	listenAddr := net.JoinHostPort(nodeConf.Host, nodeConf.Port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start server on %s: %w", listenAddr, err)
	}
	defer listener.Close()

	fmt.Printf("Node started on %s\n", listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Connection reception error: %v\n", err)
			continue
		}

		clientIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if !isAllowedIP(clientIP, nodeConf.AllowedIPs) {
			fmt.Printf("Access denied for IP: %s\n", clientIP)
			conn.Close()
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			handleConnection(conn)
		}(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Fprintf(conn, "# munin node at %s\n", nodeConf.HostName)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, lineMax), lineMax)

	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.Fields(line)

		if len(parts) == 0 {
			fmt.Fprintln(conn, "# Unknown command. Try cap, list, nodes, config, fetch, version or quit")
			continue
		}

		cmd := parts[0]
		var arg string
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {

		case "cap":
			fmt.Fprintln(conn, "cap multigraph")

		case "version":
			fmt.Fprintf(conn, "munin node version: %s\n", version)

		case "nodes":
			fmt.Fprintf(conn, "%s\n.\n", nodeConf.HostName)

		case "list":
			fmt.Fprintln(conn, listPlugins())

		case "config":
			if len(cmd) > 1 {

				output, err := executePlugin(arg, "config")
				if err != nil {
					fmt.Fprintln(conn, "# Unknown service\n.")
				} else {
					fmt.Fprintf(conn, "%s", output)
					fmt.Fprintln(conn, ".")
				}
			} else {
				fmt.Fprintln(conn, "# Unknown service\n.\n")
			}

		case "fetch":
			if len(cmd) > 1 {

				output, err := executePlugin(arg, "")
				if err != nil {
					fmt.Fprintln(conn, "# Unknown service\n.")
				} else {
					fmt.Fprintf(conn, "%s", output)
					fmt.Fprintln(conn, ".")
				}

			} else {
				fmt.Fprintln(conn, "# Unknown service\n.\n")
			}

		case "quit":
			return

		default:
			fmt.Fprintln(conn, "# Unknown command. Try cap, list, nodes, config, fetch, version or quit")
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Printf("Error reading from connection: %v", err)
	}
}

func main() {

	err := readNodeConfig(nodeConfigPath)
	if err != nil {
		fmt.Printf("Configuration loading error: %v\n", err)
		return
	}

	err = startNode()
	if err != nil {
		fmt.Printf("Node startup error: %v\n", err)
	}
}
