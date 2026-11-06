package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 夜间断网：用 Windows 计划任务在指定时间禁用/恢复网卡。
// 任务以最高权限运行，因此只在"应用/测试"时需要一次管理员授权（UAC）。
//
// 重要：网卡用 InterfaceIndex（数字）标识，不用名称——名称可能是中文，
// 跨进程传递容易因编码问题变成乱码。

const (
	nightOffDisableTask = "NeiWangTong-NightOff-Disable"
	nightOffEnableTask  = "NeiWangTong-NightOff-Enable"

	// 旧版 FlyDog 注册的任务名。重新应用计划时一并删除；在用户重新
	// 应用之前，它们仍被视为“已安装”，避免升级后显示成未启用。
	legacyNightOffDisableTask = "FlyDog-NightOff-Disable"
	legacyNightOffEnableTask  = "FlyDog-NightOff-Enable"
)

// NetAdapter 是给前端展示的网卡信息。
type NetAdapter struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Status string `json:"status"`
}

func nightOffDir() string { return configDir() }

func nightOffScriptPath() string { return filepath.Join(nightOffDir(), "nightoff.ps1") }
func nightOffResultPath() string { return filepath.Join(nightOffDir(), "nightoff-result.txt") }
func nightOffLogPath() string    { return filepath.Join(nightOffDir(), "nightoff.log") }

func quotePS(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func runPowerShell(script string) (string, error) {
	// Force UTF-8 stdout so non-ASCII adapter names survive the pipe.
	full := "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; " + script
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", full)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runElevated runs a .ps1 file elevated and waits for it to finish.
//
// -ArgumentList is passed as one string with embedded double quotes: when an
// array is used, Start-Process joins the elements with spaces without quoting
// them, so a script path containing spaces (e.g. "C:\Users\John Doe\...")
// would be split apart.
func runElevated(scriptPath string) error {
	args := "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File \"" + scriptPath + "\""
	inner := fmt.Sprintf(
		"Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -WindowStyle Hidden -ArgumentList %s",
		quotePS(args),
	)
	_, err := runPowerShell(inner)
	return err
}

// ListAdapters 列出网卡（含 InterfaceIndex）。
func (a *App) ListAdapters() ([]NetAdapter, error) {
	out, err := runPowerShell(
		"@(Get-NetAdapter | ForEach-Object { [pscustomobject]@{ index=$_.ifIndex; name=$_.Name; desc=$_.InterfaceDescription; status=$_.Status } }) | ConvertTo-Json -Compress -Depth 3",
	)
	if err != nil {
		return nil, fmt.Errorf("查询网卡失败: %v %s", err, strings.TrimSpace(out))
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "null" {
		return []NetAdapter{}, nil
	}
	var raw []NetAdapter
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		var one NetAdapter
		if err2 := json.Unmarshal([]byte(out), &one); err2 != nil {
			return nil, fmt.Errorf("解析网卡列表失败: %v", err)
		}
		return []NetAdapter{one}, nil
	}
	return raw, nil
}

// adapterStatus 返回指定网卡当前状态（Disabled/Up/...）。
func adapterStatus(index int) string {
	out, _ := runPowerShell(fmt.Sprintf("(Get-NetAdapter -InterfaceIndex %d).Status", index))
	return strings.TrimSpace(out)
}

// writeNightOffScript 生成执行网卡开关的脚本（按 InterfaceIndex 操作）。
func writeNightOffScript() error {
	if err := os.MkdirAll(nightOffDir(), 0o755); err != nil {
		return err
	}
	script := `param([string]$Action, [int]$Index)
$log = ` + quotePS(nightOffLogPath()) + `
function Log($m) { Add-Content -Path $log -Value ("{0}  {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $m) -Encoding UTF8 }
$ad = Get-NetAdapter -InterfaceIndex $Index -ErrorAction SilentlyContinue
if (-not $ad) { Log "adapter index=$Index not found"; exit 1 }
$name = $ad.Name
if ($Action -eq 'disable') {
    try { $ad | Disable-NetAdapter -Confirm:$false -ErrorAction Stop } catch { Log ("Disable-NetAdapter error: " + $_.Exception.Message) }
    Start-Sleep -Seconds 1
    if ((Get-NetAdapter -InterfaceIndex $Index).Status -ne 'Disabled') {
        netsh interface set interface name="$name" admin=disable | Out-Null
    }
} else {
    try { $ad | Enable-NetAdapter -Confirm:$false -ErrorAction Stop } catch { Log ("Enable-NetAdapter error: " + $_.Exception.Message) }
    Start-Sleep -Seconds 1
    if ((Get-NetAdapter -InterfaceIndex $Index).Status -eq 'Disabled') {
        netsh interface set interface name="$name" admin=enable | Out-Null
    }
}
$st = (Get-NetAdapter -InterfaceIndex $Index).Status
Log ("{0} [{1}] '{2}' -> {3}" -f $Action, $Index, $name, $st)
`
	return os.WriteFile(nightOffScriptPath(), []byte(script), 0o644)
}

// InstallNightOff 注册（或更新）两条计划任务：start 禁用网卡，end 恢复网卡。
func (a *App) InstallNightOff(index int, start, end string) error {
	if index <= 0 {
		return errors.New("请先选择网卡")
	}
	var ok bool
	if start, ok = normalizeHHMM(start); !ok {
		return errors.New("断网时间格式应为 HH:mm")
	}
	if end, ok = normalizeHHMM(end); !ok {
		return errors.New("恢复时间格式应为 HH:mm")
	}
	if err := writeNightOffScript(); err != nil {
		return err
	}
	_ = os.Remove(nightOffResultPath())

	setup := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
try {
    $ps1 = %s
    $idx = %d
    Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
    $principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    $argOff = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + $ps1 + '" disable ' + $idx
    $argOn  = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + $ps1 + '" enable ' + $idx
    $aOff = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $argOff
    $aOn  = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $argOn
    $tOff = New-ScheduledTaskTrigger -Daily -At ([datetime]::ParseExact('%s','HH:mm',$null))
    $tOn  = New-ScheduledTaskTrigger -Daily -At ([datetime]::ParseExact('%s','HH:mm',$null))
    Register-ScheduledTask -TaskName '%s' -Action $aOff -Trigger $tOff -Principal $principal -Settings $settings -Force | Out-Null
    Register-ScheduledTask -TaskName '%s' -Action $aOn  -Trigger $tOn  -Principal $principal -Settings $settings -Force | Out-Null
    'OK' | Set-Content -Path %s -Encoding UTF8
} catch {
    ('ERROR: ' + $_.Exception.Message) | Set-Content -Path %s -Encoding UTF8
}
`,
		quotePS(nightOffScriptPath()), index,
		nightOffDisableTask, nightOffEnableTask,
		legacyNightOffDisableTask, legacyNightOffEnableTask,
		start, end,
		nightOffDisableTask, nightOffEnableTask,
		quotePS(nightOffResultPath()), quotePS(nightOffResultPath()),
	)

	setupPath := filepath.Join(nightOffDir(), "nightoff-setup.ps1")
	if err := os.MkdirAll(nightOffDir(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(setupPath, []byte(setup), 0o644); err != nil {
		return err
	}
	if err := runElevated(setupPath); err != nil {
		return fmt.Errorf("未获得管理员授权或执行失败: %v", err)
	}
	return readNightOffResult()
}

// RemoveNightOff 删除计划任务。
func (a *App) RemoveNightOff() error {
	_ = os.Remove(nightOffResultPath())
	script := fmt.Sprintf(`$ErrorActionPreference = 'SilentlyContinue'
Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName '%s' -Confirm:$false -ErrorAction SilentlyContinue
'OK' | Set-Content -Path %s -Encoding UTF8
`, nightOffDisableTask, nightOffEnableTask,
		legacyNightOffDisableTask, legacyNightOffEnableTask,
		quotePS(nightOffResultPath()))
	p := filepath.Join(nightOffDir(), "nightoff-remove.ps1")
	if err := os.MkdirAll(nightOffDir(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(script), 0o644); err != nil {
		return err
	}
	if err := runElevated(p); err != nil {
		return fmt.Errorf("未获得管理员授权或执行失败: %v", err)
	}
	return readNightOffResult()
}

// NightOffStatus 报告计划任务是否已注册（兼容旧版 FlyDog 任务名）。
func (a *App) NightOffStatus() bool {
	return nightOffTaskExists(nightOffDisableTask) || nightOffTaskExists(legacyNightOffDisableTask)
}

func nightOffTaskExists(name string) bool {
	cmd := exec.Command("schtasks", "/Query", "/TN", name)
	hideWindow(cmd)
	return cmd.Run() == nil
}

// NightOffRuns 是最近一次断网 / 恢复的执行记录（来自 nightoff.log）。
type NightOffRuns struct {
	DisableTime   string `json:"disableTime"`
	DisableStatus string `json:"disableStatus"`
	EnableTime    string `json:"enableTime"`
	EnableStatus  string `json:"enableStatus"`
}

// NightOffLastRuns 从日志中解析最近一次断网与恢复的时间和结果。
func (a *App) NightOffLastRuns() NightOffRuns {
	r := parseNightOffLog(nightOffLogPath())
	// 旧版任务会继续写入旧目录的日志，直到用户重新应用计划；合并新旧
	// 日志，升级后的执行记录不会看起来“消失”。
	legacy := parseNightOffLog(filepath.Join(legacyConfigDir(), "nightoff.log"))
	return mergeNightOffRuns(r, legacy)
}

func parseNightOffLog(path string) NightOffRuns {
	var r NightOffRuns
	b, err := os.ReadFile(path)
	if err != nil {
		return r
	}
	for _, line := range strings.Split(strings.TrimPrefix(string(b), "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		ts := f[0] + " " + f[1]
		status := ""
		if i := strings.LastIndex(line, "->"); i >= 0 {
			status = strings.TrimSpace(line[i+2:])
		}
		switch f[2] {
		case "disable":
			r.DisableTime, r.DisableStatus = ts, status
		case "enable":
			r.EnableTime, r.EnableStatus = ts, status
		}
	}
	return r
}

// mergeNightOffRuns keeps the newer of each timestamp (the log format sorts
// lexicographically).
func mergeNightOffRuns(a, b NightOffRuns) NightOffRuns {
	if b.DisableTime > a.DisableTime {
		a.DisableTime, a.DisableStatus = b.DisableTime, b.DisableStatus
	}
	if b.EnableTime > a.EnableTime {
		a.EnableTime, a.EnableStatus = b.EnableTime, b.EnableStatus
	}
	return a
}

// NightOffNow 立即禁用/恢复网卡（会弹一次 UAC），并校验实际结果。
func (a *App) NightOffNow(index int, enable bool) error {
	if index <= 0 {
		return errors.New("请先选择网卡")
	}
	if err := writeNightOffScript(); err != nil {
		return err
	}
	_ = os.Remove(nightOffResultPath())
	action := "disable"
	if enable {
		action = "enable"
	}
	script := fmt.Sprintf(`$ErrorActionPreference = 'SilentlyContinue'
try {
    & %s -Action '%s' -Index %d
    $st = (Get-NetAdapter -InterfaceIndex %d).Status
    ('OK:' + $st) | Set-Content -Path %s -Encoding UTF8
} catch {
    ('ERROR: ' + $_.Exception.Message) | Set-Content -Path %s -Encoding UTF8
}
`,
		quotePS(nightOffScriptPath()), action, index, index,
		quotePS(nightOffResultPath()), quotePS(nightOffResultPath()),
	)
	if err := os.MkdirAll(nightOffDir(), 0o755); err != nil {
		return err
	}
	p := filepath.Join(nightOffDir(), "nightoff-now.ps1")
	if err := os.WriteFile(p, []byte(script), 0o644); err != nil {
		return err
	}
	if err := runElevated(p); err != nil {
		return fmt.Errorf("未获得管理员授权或执行失败: %v", err)
	}
	if err := readNightOffResult(); err != nil {
		return err
	}

	// Verify the adapter really changed state.
	st := adapterStatus(index)
	if enable {
		if strings.EqualFold(st, "Disabled") {
			return fmt.Errorf("恢复失败：网卡仍为 Disabled（可能被组策略/安全软件阻止）")
		}
	} else if !strings.EqualFold(st, "Disabled") {
		return fmt.Errorf("禁用失败：网卡当前状态为 %q", st)
	}
	return nil
}

func readNightOffResult() error {
	b, err := os.ReadFile(nightOffResultPath())
	if err != nil {
		return errors.New("未收到执行结果（可能取消了管理员授权）")
	}
	return parseNightOffResult(b)
}

// parseNightOffResult interprets the result file written by the elevated
// script. Windows PowerShell 5.1 writes Set-Content -Encoding UTF8 with a BOM,
// so strip it before looking at the payload (strings.TrimSpace does not remove
// U+FEFF).
func parseNightOffResult(b []byte) error {
	s := strings.TrimSpace(strings.TrimPrefix(string(b), "\ufeff"))
	if strings.HasPrefix(s, "OK") {
		return nil
	}
	msg := strings.TrimSpace(strings.TrimPrefix(s, "ERROR:"))
	if msg == "" {
		msg = "\u591c\u95f4\u65ad\u7f51\u64cd\u4f5c\u5931\u8d25"
	}
	return errors.New(msg)
}

// normalizeHHMM validates a HH:mm string and returns its canonical form.
// time.Parse rejects trailing garbage ("10:30abc") that fmt.Sscanf accepts.
func normalizeHHMM(s string) (string, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return "", false
	}
	return t.Format("15:04"), true
}
