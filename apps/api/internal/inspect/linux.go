//go:build linux

package inspect

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func cleanRoot(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return filepath.Clean(value)
}

func collect(proc, root string, prev *prevCPU) Report {
	rep := Report{
		CollectedAt:  time.Now().UTC(),
		Listeners:    []Listen{},
		Deleted:      []Deleted{},
		RAID:         []RAID{},
		NVMe:         []NVMe{},
		KernelEvents: []KEvent{},
		Cron:         []Cron{},
		IPMINote:     "BMC sensor readings are not collected. ipmitool is not executed.",
	}
	rep.Hostname = hostname()
	rep.FQDN = rep.Hostname
	rep.ID = shortHost(rep.Hostname)
	rep.Kernel = readTrim(filepath.Join(proc, "sys/kernel/osrelease"))
	rep.OS = osRelease(filepath.Join(root, "etc/os-release"))
	rep.CPUModel, rep.Hypervisor = cpuInfo(filepath.Join(proc, "cpuinfo"))
	rep.UptimeSeconds = uptime(filepath.Join(proc, "uptime"))
	rep.Load1, rep.Load5, rep.Load15 = loadavg(filepath.Join(proc, "loadavg"))
	rep.Threads = countCPUs(filepath.Join(proc, "stat"))
	rep.Cores = rep.Threads
	if rep.Cores == 0 {
		rep.Cores = 1
	}
	vendor := readTrim(filepath.Join(root, "sys/class/dmi/id/sys_vendor"))
	product := readTrim(filepath.Join(root, "sys/class/dmi/id/product_name"))
	rep.Provider = joinDot(vendor, product)
	rep.Virt, rep.Kind = virtLabel(rep.Hypervisor, vendor, product)
	if rep.Provider == "" {
		rep.Provider = rep.Virt
	}
	rep.IP = firstIP()
	rep.RebootPending = fileExists(filepath.Join(root, "var/run/reboot-required")) || fileExists("/var/run/reboot-required")
	rep.PSI = readPSI(proc)
	rep.AgentCPU = selfCPU(proc, prev)
	rep.AgentRSS = selfRSS(proc)
	rep.RAID, rep.RAIDNote = readRAID(filepath.Join(proc, "mdstat"))
	rep.NVMe, rep.NVMeNote = readNVMe(filepath.Join(root, "sys/class/nvme"))
	if _, err := os.Stat("/dev/ipmi0"); err == nil {
		rep.IPMINote = "A BMC device node exists, but sensor readings are not collected. ipmitool is not executed."
	}
	rep.Listeners, rep.Deleted, rep.ListenNote, rep.DeletedNote = readProcFDs(proc)
	rep.KernelEvents, rep.KernelNote = readKernelLog(root)
	rep.SSH = readSSH(root)
	rep.Cron, rep.CronNote = readCron(root)
	rep.Mail = readMail(root)
	return rep
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

func shortHost(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "host"
	}
	if i := strings.IndexByte(name, '.'); i > 0 {
		return name[:i]
	}
	return name
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func osRelease(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var name, pretty string
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "PRETTY_NAME":
			pretty = value
		case "NAME":
			name = value
		}
	}
	if pretty != "" {
		return pretty
	}
	return name
}

func cpuInfo(path string) (model string, hypervisor bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Model") {
			if model == "" {
				_, value, ok := strings.Cut(line, ":")
				if ok {
					model = strings.TrimSpace(value)
				}
			}
		}
		if strings.HasPrefix(line, "flags") || strings.HasPrefix(line, "Features") {
			if strings.Contains(line, " hypervisor") || strings.HasSuffix(line, "hypervisor") {
				hypervisor = true
			}
		}
		if model != "" && hypervisor {
			break
		}
	}
	return model, hypervisor
}

func uptime(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return v
}

func loadavg(path string) (float64, float64, float64) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	a, _ := strconv.ParseFloat(fields[0], 64)
	c, _ := strconv.ParseFloat(fields[1], 64)
	d, _ := strconv.ParseFloat(fields[2], 64)
	return a, c, d
}

func countCPUs(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if len(name) > 3 && strings.HasPrefix(name, "cpu") && isDigits(name[3:]) {
			n++
		}
	}
	return n
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func virtLabel(hypervisor bool, vendor, product string) (string, string) {
	blob := strings.ToLower(vendor + " " + product)
	switch {
	case strings.Contains(blob, "kvm") || strings.Contains(blob, "qemu"):
		return "KVM guest", "vps"
	case strings.Contains(blob, "vmware"):
		return "VMware guest", "vps"
	case strings.Contains(blob, "virtualbox"):
		return "VirtualBox guest", "vps"
	case strings.Contains(blob, "microsoft") && strings.Contains(blob, "virtual"):
		return "Hyper-V guest", "vps"
	case strings.Contains(blob, "xen"):
		return "Xen guest", "vps"
	case hypervisor:
		return "Virtual machine", "vps"
	default:
		return "Bare metal", "dedicated"
	}
}

func joinDot(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	switch {
	case a != "" && b != "":
		return a + " · " + b
	case a != "":
		return a
	default:
		return b
	}
}

func firstIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	var fallback string
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP == nil || ipnet.IP.IsLoopback() {
			continue
		}
		ip := ipnet.IP
		if ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.IsGlobalUnicast() && !ip.IsPrivate() {
			return ip.String()
		}
		if fallback == "" && ip.IsGlobalUnicast() {
			fallback = ip.String()
		}
	}
	return fallback
}

func selfRSS(proc string) uint64 {
	b, err := os.ReadFile(filepath.Join(proc, "self/statm"))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 2 {
		return 0
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return pages * uint64(os.Getpagesize())
}

func readPSI(proc string) PSI {
	mem, okMem := readPressure(filepath.Join(proc, "pressure/memory"))
	io, okIO := readPressure(filepath.Join(proc, "pressure/io"))
	if !okMem && !okIO {
		return PSI{}
	}
	return PSI{Available: true, MemorySome: mem[0], MemoryFull: mem[1], IOSome: io[0]}
}

func readPressure(path string) ([2]float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return [2]float64{}, false
	}
	var out [2]float64
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var avg float64
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "avg10=") {
				avg, _ = strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
			}
		}
		switch fields[0] {
		case "some":
			out[0] = avg
			found = true
		case "full":
			out[1] = avg
			found = true
		}
	}
	return out, found
}

func selfCPU(proc string, prev *prevCPU) *float64 {
	b, err := os.ReadFile(filepath.Join(proc, "self/stat"))
	if err != nil || prev == nil {
		return nil
	}
	fields := strings.Fields(string(b))
	if len(fields) < 15 {
		return nil
	}
	utime, err1 := strconv.ParseUint(fields[13], 10, 64)
	stime, err2 := strconv.ParseUint(fields[14], 10, 64)
	if err1 != nil || err2 != nil {
		return nil
	}
	ticks := utime + stime
	now := time.Now()
	if prev.at.IsZero() {
		prev.at = now
		prev.ticks = ticks
		return nil
	}
	dt := now.Sub(prev.at).Seconds()
	prev.at = now
	delta := ticks - prev.ticks
	prev.ticks = ticks
	if dt <= 0 {
		return nil
	}
	const hz = 100.0
	cores := countCPUs(filepath.Join(proc, "stat"))
	if cores < 1 {
		cores = 1
	}
	pct := (float64(delta) / hz) / dt / float64(cores) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	pct = float64(int(pct*10)) / 10
	return &pct
}

func readRAID(path string) ([]RAID, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return []RAID{}, "RAID state is not visible. /proc/mdstat could not be read."
	}
	text := string(b)
	if !strings.Contains(text, "md") {
		return []RAID{}, "No md RAID arrays are active."
	}
	var out []RAID
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "md") {
			continue
		}
		item := RAID{Name: fields[0], State: fields[2], Level: fields[3]}
		if i+1 < len(lines) {
			if start := strings.Index(lines[i+1], "["); start >= 0 {
				end := strings.Index(lines[i+1][start:], "]")
				if end > 0 {
					item.Members = lines[i+1][start : start+end+1]
				}
			}
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return []RAID{}, "No md RAID arrays are active."
	}
	return out, ""
}

func readNVMe(dir string) ([]NVMe, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []NVMe{}, "NVMe devices are not visible in sysfs."
	}
	var out []NVMe
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "nvme") {
			continue
		}
		base := filepath.Join(dir, entry.Name())
		item := NVMe{Name: entry.Name(), Model: readTrim(filepath.Join(base, "model"))}
		if temp := nvmeTemp(base); temp != nil {
			item.TempC = temp
		}
		out = append(out, item)
		if len(out) == 8 {
			break
		}
	}
	if len(out) == 0 {
		return []NVMe{}, "No NVMe controllers are visible in sysfs."
	}
	return out, ""
}

func nvmeTemp(dir string) *float64 {
	matches, _ := filepath.Glob(filepath.Join(dir, "hwmon*", "temp*_input"))
	if len(matches) == 0 {
		return nil
	}
	b, err := os.ReadFile(matches[0])
	if err != nil {
		return nil
	}
	milli, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return nil
	}
	c := milli / 1000
	return &c
}

func readKernelLog(root string) ([]KEvent, string) {
	for _, rel := range []string{"var/log/kern.log", "var/log/syslog"} {
		path := filepath.Join(root, rel)
		events := parseKernelTail(path)
		if events == nil {
			continue
		}
		if len(events) == 0 {
			return []KEvent{}, ""
		}
		return events, ""
	}
	return []KEvent{}, "Kernel logs are not readable by this process."
}

func parseKernelTail(path string) []KEvent {
	b, err := readTail(path, 96<<10)
	if err != nil {
		return nil
	}
	var out []KEvent
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		lower := strings.ToLower(line)
		tone := ""
		lane := ""
		switch {
		case strings.Contains(lower, "out of memory"):
			tone, lane = "critical", "OOM killer"
		case strings.Contains(lower, "blocked for more than"):
			tone, lane = "warning", "Kernel warnings"
		case strings.Contains(lower, "nvme") && (strings.Contains(lower, "error") || strings.Contains(lower, "timeout")):
			tone, lane = "warning", "Storage / RAID"
		case strings.Contains(lower, "syn flooding") || strings.Contains(lower, "link up"):
			tone, lane = "warning", "Network"
		}
		if tone == "" {
			continue
		}
		if strings.Contains(lower, "link up") {
			tone = "info"
		}
		out = append(out, KEvent{At: time.Now().UTC(), Tone: tone, Lane: lane, Label: trimLine(line)})
		if len(out) == 12 {
			break
		}
	}
	return out
}

func readSSH(root string) SSHReport {
	var body []byte
	var err error
	for _, rel := range []string{"var/log/auth.log", "var/log/secure"} {
		body, err = readTail(filepath.Join(root, rel), 256<<10)
		if err == nil {
			break
		}
	}
	if err != nil {
		return SSHReport{Note: "SSH authentication logs are not readable by this process."}
	}
	type agg struct {
		fails int
		users map[string]struct{}
	}
	byIP := map[string]*agg{}
	order := []string{}
	failed := 0
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "Failed password") && !strings.Contains(line, "Invalid user") && !strings.Contains(line, "Failed publickey") {
			continue
		}
		failed++
		fields := strings.Fields(line)
		ip := ""
		user := ""
		for i, field := range fields {
			if field == "from" && i+1 < len(fields) {
				ip = strings.Trim(fields[i+1], ":")
			}
			if (field == "user" || field == "for") && i+1 < len(fields) && user == "" {
				user = strings.Trim(fields[i+1], ":")
			}
		}
		if ip == "" || net.ParseIP(ip) == nil {
			continue
		}
		item := byIP[ip]
		if item == nil {
			item = &agg{users: map[string]struct{}{}}
			byIP[ip] = item
			order = append(order, ip)
		}
		item.fails++
		if user != "" && user != "invalid" {
			item.users[user] = struct{}{}
		}
	}
	sources := []SSHSource{}
	for _, ip := range order {
		item := byIP[ip]
		users := make([]string, 0, len(item.users))
		for user := range item.users {
			users = append(users, user)
			if len(users) == 4 {
				break
			}
		}
		sources = append(sources, SSHSource{IP: ip, Fails: item.fails, Users: strings.Join(users, ", ")})
		if len(sources) == 8 {
			break
		}
	}
	return SSHReport{Available: true, Failed: failed, Sources: sources}
}

func readCron(root string) ([]Cron, string) {
	var out []Cron
	paths := []string{filepath.Join(root, "etc/crontab")}
	entries, err := os.ReadDir(filepath.Join(root, "etc/cron.d"))
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			paths = append(paths, filepath.Join(root, "etc/cron.d", entry.Name()))
			if len(paths) > 12 {
				break
			}
		}
	}
	saw := false
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil || len(b) > 256<<10 {
			continue
		}
		saw = true
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 7 || strings.Contains(fields[0], "=") {
				continue
			}
			out = append(out, Cron{
				Schedule: strings.Join(fields[:5], " "),
				Command:  strings.Join(fields[6:], " "),
				Source:   filepath.Base(path),
			})
			if len(out) == 20 {
				return out, ""
			}
		}
	}
	if !saw {
		return []Cron{}, "Crontab files are not readable by this process."
	}
	if len(out) == 0 {
		return []Cron{}, ""
	}
	return out, ""
}

func readMail(root string) MailReport {
	b, err := readTail(filepath.Join(root, "var/log/mail.log"), 256<<10)
	if err != nil {
		return MailReport{Note: "Mail logs are not readable by this process."}
	}
	report := MailReport{Available: true}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, "status=sent"):
			report.Sent++
		case strings.Contains(line, "status=deferred"):
			report.Deferred++
		case strings.Contains(line, "status=bounced"):
			report.Bounced++
		}
	}
	return report
}

func readTail(path string, limit int64) ([]byte, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || strings.Contains(path, "..") {
		return nil, os.ErrInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, os.ErrInvalid
	}
	start := int64(0)
	if info.Size() > limit {
		start = info.Size() - limit
	}
	buf := make([]byte, info.Size()-start)
	_, err = f.ReadAt(buf, start)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return nil, err
	}
	if i := bytes.IndexByte(buf, '\n'); i >= 0 && start > 0 {
		buf = buf[i+1:]
	}
	return buf, nil
}

func trimLine(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 180 {
		return line[:180]
	}
	return line
}

type rawListen struct {
	Listen
	inode string
}

func readProcFDs(proc string) ([]Listen, []Deleted, string, string) {
	raw := parseListeners(proc)
	owners := map[string]owner{}
	var deleted []Deleted
	entries, err := os.ReadDir(proc)
	if err != nil {
		return []Listen{}, []Deleted{}, "Process owners could not be read.", "Open files could not be read."
	}
	scanned := 0
	for _, entry := range entries {
		if scanned > 400 || !isDigits(entry.Name()) {
			continue
		}
		pid, _ := strconv.Atoi(entry.Name())
		base := filepath.Join(proc, entry.Name())
		comm := readTrim(filepath.Join(base, "comm"))
		uid := uidOf(filepath.Join(base, "status"))
		fds, err := os.ReadDir(filepath.Join(base, "fd"))
		if err != nil {
			continue
		}
		scanned++
		if len(fds) > 64 {
			fds = fds[:64]
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(base, "fd", fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
				inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
				owners[inode] = owner{pid: pid, comm: comm, uid: uid}
			}
			if strings.Contains(target, "(deleted)") && len(deleted) < 30 {
				n, _ := strconv.Atoi(fd.Name())
				deleted = append(deleted, Deleted{
					PID: pid, Process: comm, Path: target, Bytes: fdSize(filepath.Join(base, "fd", fd.Name())),
				})
				_ = n
			}
		}
	}
	users := passwdUsers()
	listeners := make([]Listen, 0, len(raw))
	for _, item := range raw {
		if own, ok := owners[item.inode]; ok {
			item.PID = own.pid
			item.Process = own.comm
			item.User = users[own.uid]
			if item.User == "" && own.uid >= 0 {
				item.User = strconv.Itoa(own.uid)
			}
		}
		listeners = append(listeners, item.Listen)
	}
	note := ""
	if len(listeners) == 0 {
		note = "No listening sockets were visible in /proc/net."
	}
	deletedNote := ""
	if len(deleted) == 0 {
		deletedNote = "No deleted-but-open files were visible to this user."
	}
	return listeners, deleted, note, deletedNote
}

type owner struct {
	pid  int
	comm string
	uid  int
}

func uidOf(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				n, _ := strconv.Atoi(fields[1])
				return n
			}
		}
	}
	return -1
}

func fdSize(path string) uint64 {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0
	}
	if st.Size < 0 {
		return 0
	}
	return uint64(st.Size)
}

func parseListeners(proc string) []rawListen {
	var out []rawListen
	out = append(out, parseNet(filepath.Join(proc, "net/tcp"), "tcp", false)...)
	out = append(out, parseNet(filepath.Join(proc, "net/tcp6"), "tcp", true)...)
	out = append(out, parseNet(filepath.Join(proc, "net/udp"), "udp", false)...)
	out = append(out, parseNet(filepath.Join(proc, "net/udp6"), "udp", true)...)
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func parseNet(path, proto string, v6 bool) []rawListen {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	var out []rawListen
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		if proto == "tcp" && fields[3] != "0A" {
			continue
		}
		ip, port, ok := parseHexAddr(fields[1], v6)
		if !ok {
			continue
		}
		out = append(out, rawListen{
			Listen: Listen{Proto: proto, Addr: net.JoinHostPort(ip, strconv.Itoa(port)), Exposure: exposure(ip)},
			inode:  fields[9],
		})
	}
	return out
}

func parseHexAddr(field string, v6 bool) (string, int, bool) {
	ipHex, portHex, ok := strings.Cut(field, ":")
	if !ok {
		return "", 0, false
	}
	port64, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", 0, false
	}
	if !v6 {
		if len(ipHex) != 8 {
			return "", 0, false
		}
		v, err := strconv.ParseUint(ipHex, 16, 32)
		if err != nil {
			return "", 0, false
		}
		ip := net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
		return ip.String(), int(port64), true
	}
	if len(ipHex) != 32 {
		return "", 0, false
	}
	buf := make([]byte, 16)
	for i := 0; i < 16; i += 4 {
		word, err := strconv.ParseUint(ipHex[i*2:(i+4)*2], 16, 32)
		if err != nil {
			return "", 0, false
		}
		buf[i] = byte(word)
		buf[i+1] = byte(word >> 8)
		buf[i+2] = byte(word >> 16)
		buf[i+3] = byte(word >> 24)
	}
	return net.IP(buf).String(), int(port64), true
}

func exposure(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "public"
	}
	if parsed.IsUnspecified() {
		return "public"
	}
	if parsed.IsLoopback() {
		return "local"
	}
	if parsed.IsPrivate() {
		return "private"
	}
	return "public"
}

func passwdUsers() map[int]string {
	out := map[int]string{}
	b, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		out[uid] = parts[0]
	}
	return out
}
