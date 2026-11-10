//go:build !linux

package inspect

import "time"

func cleanRoot(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func collect(proc, root string, prev *prevCPU) Report {
	_ = proc
	_ = root
	_ = prev
	return Report{
		CollectedAt:  time.Now().UTC(),
		Kind:         "unknown",
		Virt:         "Unavailable",
		ListenNote:   "Socket inventory is collected on Linux.",
		DeletedNote:  "Open-file inventory is collected on Linux.",
		RAIDNote:     "RAID state is collected from /proc/mdstat on Linux.",
		NVMeNote:     "NVMe sysfs is read on Linux.",
		IPMINote:     "BMC sensors are not read on this build. ipmitool is not executed.",
		KernelNote:   "Kernel logs are read on Linux when a log file is readable.",
		SSH:          SSHReport{Note: "SSH logs are read on Linux when auth.log or secure is readable."},
		CronNote:     "Crontabs are read on Linux.",
		Mail:         MailReport{Note: "Mail logs are read on Linux when mail.log is readable."},
		Listeners:    []Listen{},
		Deleted:      []Deleted{},
		RAID:         []RAID{},
		NVMe:         []NVMe{},
		KernelEvents: []KEvent{},
		Cron:         []Cron{},
	}
}
