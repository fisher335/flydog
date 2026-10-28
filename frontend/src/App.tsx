import {useEffect, useMemo, useRef, useState} from 'react';
import './App.css';
import {Events} from '@wailsio/runtime';
import {App as AppSvc} from '../bindings/flydog';

const {
    AddPeer,
    ClearHistory,
    GetAutostart,
    GetMembers,
    InstallNightOff,
    IsRunning,
    ListAdapters,
    LoadConfig,
    LoadHistory,
    NightOffLastRuns,
    NightOffNow,
    NightOffStatus,
    OpenDownloadDir,
    OpenYou,
    PickFiles,
    PickFolder,
    RemoveNightOff,
    Rescan,
    SaveConfig,
    SaveHistory,
    Scan,
    SendFiles,
    SendKnock,
    SendText,
    SendTyping,
    SetName,
    SetAutostart,
    Start,
    Stop,
} = AppSvc;

// Wails v3 delivers a WailsEvent object; unwrap its `.data` payload.
function onEvent(name: string, cb: (data: any) => void): () => void {
    return Events.On(name, (ev: any) => cb(ev && typeof ev === 'object' && 'data' in ev ? ev.data : ev));
}

interface Member {
    name: string;
    group?: string;
    host: string;
    ip: string;
    port: number;
    absence: boolean;
    utf8: boolean;
}

interface ChatMsg {
    id: number;
    dir: 'in' | 'out';
    kind: string;
    text: string;
    files: string[];
    time: string;
}

interface Transfer {
    id: string;
    direction: string;
    peer: string;
    name: string;
    size: number;
    done: number;
    state: string;
    error?: string;
}

interface Toast {
    id: number;
    title: string;
    body: string;
}

const SETTINGS_TABS = [
    {key: 'profile', icon: '👤', label: '个人'},
    {key: 'files', icon: '📁', label: '文件与记录'},
    {key: 'network', icon: '🌐', label: '网络'},
    {key: 'system', icon: '⚙️', label: '系统'},
];

const EMOJIS = [
    '😀', '😁', '😂', '🤣', '😊', '😍', '😘', '😜', '🤔', '😎', '😭', '😡',
    '👍', '👎', '👌', '🙏', '👏', '💪', '🎉', '🔥', '💯', '❤️', '💔', '🌹',
    '🍀', '☕', '🍺', '🎁', '⭐', '✅', '❌', '⚠️', '😴', '🤝', '🤗', '😇',
    '🥳', '😢', '😅', '🙄', '🚀', '🐶', '📎', '📁', '🖥️', '📱', '⏰', '🎵',
];

function peerKey(m: Member): string {
    return `${m.ip}:${m.port}`;
}

function hashHue(s: string): number {
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
    return h % 360;
}

const AVATAR_COUNT = 24;

function hashIndex(name: string): number {
    return hashHue(name || '?') % AVATAR_COUNT;
}

// BuiltinAvatar renders a minimal geometric avatar as inline SVG: a soft
// gradient background with an abstract mark. No image assets, no cartoon faces.
const AVATAR_PALETTES: [string, string][] = [
    ['#6366f1', '#8b5cf6'],
    ['#0ea5e9', '#22d3ee'],
    ['#f97316', '#f59e0b'],
    ['#ec4899', '#f43f5e'],
    ['#10b981', '#34d399'],
    ['#64748b', '#94a3b8'],
    ['#8b5cf6', '#d946ef'],
    ['#0f766e', '#14b8a6'],
];

function BuiltinAvatar({index, size}: {index: number; size: number}) {
    const i = ((index % AVATAR_COUNT) + AVATAR_COUNT) % AVATAR_COUNT;
    const [c1, c2] = AVATAR_PALETTES[i % AVATAR_PALETTES.length];
    const shape = i % 6;
    const gid = `av-${i}`;
    return (
        <svg width={size} height={size} viewBox="0 0 100 100" style={{display: 'block'}}>
            <defs>
                <linearGradient id={gid} x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0%" stopColor={c1} />
                    <stop offset="100%" stopColor={c2} />
                </linearGradient>
            </defs>
            <rect width="100" height="100" fill={`url(#${gid})`} />
            {shape === 0 && (
                <g fill="none" stroke="#fff" strokeOpacity="0.9" strokeWidth="2.6">
                    <circle cx="50" cy="50" r="26" />
                    <circle cx="50" cy="50" r="16" />
                    <circle cx="50" cy="50" r="6" fill="#fff" fillOpacity="0.95" stroke="none" />
                </g>
            )}
            {shape === 1 && (
                <>
                    <rect x="32" y="32" width="36" height="36" rx="7" transform="rotate(45 50 50)" fill="none" stroke="#fff" strokeOpacity="0.9" strokeWidth="2.8" />
                    <circle cx="50" cy="50" r="6" fill="#fff" fillOpacity="0.95" />
                </>
            )}
            {shape === 2 && (
                <g fill="none" stroke="#fff" strokeOpacity="0.9" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M34 34 L50 50 L34 66" />
                    <path d="M54 34 L70 50 L54 66" />
                </g>
            )}
            {shape === 3 && (
                <g fill="none" stroke="#fff" strokeOpacity="0.85" strokeWidth="2.8" strokeLinecap="round">
                    <path d="M24 40 q7 -9 13 0 t13 0 t13 0 t13 0" />
                    <path d="M24 52 q7 -9 13 0 t13 0 t13 0 t13 0" />
                    <path d="M24 64 q7 -9 13 0 t13 0 t13 0 t13 0" />
                </g>
            )}
            {shape === 4 && (
                <g fill="#fff" fillOpacity="0.9">
                    {[0, 1, 2].map((r) =>
                        [0, 1, 2].map((c) => (
                            <circle key={`${r}-${c}`} cx={34 + c * 16} cy={34 + r * 16} r="3.8" />
                        )),
                    )}
                </g>
            )}
            {shape === 5 && (
                <g fill="none" stroke="#fff" strokeOpacity="0.9" strokeWidth="4" strokeLinecap="round">
                    <path d="M28 64 a22 22 0 0 1 44 0" />
                    <path d="M50 42 V30" />
                    <circle cx="50" cy="42" r="4.5" fill="#fff" stroke="none" />
                </g>
            )}
        </svg>
    );
}

function Avatar({src, name, size, className}: {src?: string; name: string; size: number; className?: string}) {
    const cls = 'avatar' + (className ? ' ' + className : '');
    if (src && src.startsWith('data:')) {
        return <img className={cls + ' img'} src={src} alt="" style={{width: size, height: size}} />;
    }
    let idx: number;
    if (src && src.startsWith('builtin:')) {
        idx = parseInt(src.slice(8), 10);
        if (Number.isNaN(idx)) idx = hashIndex(name);
    } else {
        idx = hashIndex(name);
    }
    return (
        <div className={cls} style={{width: size, height: size}}>
            <BuiltinAvatar index={idx} size={size} />
        </div>
    );
}

function fmtSize(n: number): string {
    if (!n) return '0 B';
    const u = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(n) / Math.log(1024));
    return (n / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + ' ' + u[i];
}

function clock(): string {
    const d = new Date();
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

function beep(): void {
    try {
        const Ctx = (window as any).AudioContext || (window as any).webkitAudioContext;
        const ctx = new Ctx();
        const o = ctx.createOscillator();
        const g = ctx.createGain();
        o.connect(g);
        g.connect(ctx.destination);
        o.type = 'sine';
        o.frequency.value = 880;
        g.gain.setValueAtTime(0.0001, ctx.currentTime);
        g.gain.exponentialRampToValueAtTime(0.25, ctx.currentTime + 0.01);
        g.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.35);
        o.start();
        o.stop(ctx.currentTime + 0.36);
    } catch {
        /* ignore */
    }
}

function sysNotify(title: string, body: string): void {
    try {
        if ('Notification' in window && Notification.permission === 'granted') {
            new Notification(title, {body});
        }
    } catch {
        /* ignore */
    }
}

function App() {
    const [running, setRunning] = useState(false);
    const [name, setName] = useState('');
    const [downloadDir, setDownloadDir] = useState('');
    const [autoReceive, setAutoReceive] = useState(true);

    const [members, setMembers] = useState<Member[]>([]);
    const [selectedKey, setSelectedKey] = useState('');
    const [convs, setConvs] = useState<Record<string, ChatMsg[]>>({});
    const [unread, setUnread] = useState<Record<string, number>>({});
    const [typing, setTyping] = useState<Record<string, boolean>>({});
    const [transfers, setTransfers] = useState<Record<string, Transfer>>({});

    const [input, setInput] = useState('');
    const [toasts, setToasts] = useState<Toast[]>([]);
    const [shake, setShake] = useState(false);
    const [error, setError] = useState('');

    const [showSettings, setShowSettings] = useState(false);
    const [settingsTab, setSettingsTab] = useState('profile');
    const [showTransfers, setShowTransfers] = useState(true);
    const [search, setSearch] = useState('');
    const [peerIp, setPeerIp] = useState('');
    const [scanTarget, setScanTarget] = useState('172.17.32.');
    const [emojiOpen, setEmojiOpen] = useState(false);
    const [myAvatar, setMyAvatar] = useState('');
    const [showAvatars, setShowAvatars] = useState(false);
    const [trayMode, setTrayMode] = useState(true);
    const [autostart, setAutostart] = useState(false);

    // 夜间断网
    const [nightOffEnabled, setNightOffEnabled] = useState(false);
    const [nightOffAdapter, setNightOffAdapter] = useState('');
    const [nightOffStart, setNightOffStart] = useState('22:00');
    const [nightOffEnd, setNightOffEnd] = useState('06:00');
    const [adapters, setAdapters] = useState<{index: number; name: string; desc: string; status: string}[]>([]);
    const [nightOffInstalled, setNightOffInstalled] = useState(false);
    const [nightOffBusy, setNightOffBusy] = useState(false);
    const [lastRuns, setLastRuns] = useState({
        disableTime: '',
        disableStatus: '',
        enableTime: '',
        enableStatus: '',
    });
    const [theme, setTheme] = useState<string>(() => localStorage.getItem('flydog.theme') || 'light');

    const seq = useRef(0);
    const cfgRef = useRef<any>({});
    const selectedRef = useRef('');
    const focusRef = useRef(true);
    const bootRef = useRef(false);
    const historyReady = useRef(false);
    const saveTimer = useRef<number | undefined>(undefined);
    const typingTimer = useRef<number | undefined>(undefined);
    const lastTyping = useRef(0);
    const bottomRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        selectedRef.current = selectedKey;
    }, [selectedKey]);

    useEffect(() => {
        cfgRef.current = {
            name,
            downloadDir,
            autoReceive,
            avatar: myAvatar,
            minimizeToTray: trayMode,
            nightOffEnabled,
            nightOffAdapter,
            nightOffStart,
            nightOffEnd,
        };
    }, [name, downloadDir, autoReceive, myAvatar, trayMode, nightOffEnabled, nightOffAdapter, nightOffStart, nightOffEnd]);

    useEffect(() => {
        document.documentElement.dataset.theme = theme;
        localStorage.setItem('flydog.theme', theme);
    }, [theme]);

    useEffect(() => {
        const onFocus = () => (focusRef.current = true);
        const onBlur = () => (focusRef.current = false);
        window.addEventListener('focus', onFocus);
        window.addEventListener('blur', onBlur);
        return () => {
            window.removeEventListener('focus', onFocus);
            window.removeEventListener('blur', onBlur);
        };
    }, []);

    useEffect(() => {
        if (bootRef.current) return;
        bootRef.current = true;
        (async () => {
            try {
                const c = await LoadConfig();
                setName(c.name);
                setDownloadDir(c.downloadDir);
                setAutoReceive(c.autoReceive);
                setTrayMode(c.minimizeToTray);
                GetAutostart().then(setAutostart).catch(() => undefined);
                setNightOffEnabled(!!c.nightOffEnabled);
                setNightOffAdapter(c.nightOffAdapter || '');
                setNightOffStart(c.nightOffStart || '22:00');
                setNightOffEnd(c.nightOffEnd || '06:00');
                ListAdapters().then((list: any) => setAdapters(list || [])).catch(() => undefined);
                NightOffStatus().then(setNightOffInstalled).catch(() => undefined);
                NightOffLastRuns().then((r: any) => r && setLastRuns(r)).catch(() => undefined);
                let av = c.avatar;
                if (!av) av = 'builtin:' + Math.floor(Math.random() * AVATAR_COUNT);
                setMyAvatar(av);
                if (!c.avatar) {
                    SaveConfig({
                        name: c.name,
                        downloadDir: c.downloadDir,
                        autoReceive: c.autoReceive,
                        avatar: av,
                        minimizeToTray: c.minimizeToTray,
                        nightOffEnabled: c.nightOffEnabled,
                        nightOffAdapter: c.nightOffAdapter,
                        nightOffStart: c.nightOffStart,
                        nightOffEnd: c.nightOffEnd,
                    }).catch(() => undefined);
                }
                if (await IsRunning()) {
                    setRunning(true);
                } else {
                    // Auto-start the engine on launch.
                    await Start(c.name, c.downloadDir, c.autoReceive);
                    setRunning(true);
                }
            } catch (e: any) {
                setError(String(e));
            }
            GetMembers().then((m: any) => setMembers(m || []));
            LoadHistory()
                .then((h: any) => {
                    const c: Record<string, ChatMsg[]> = {};
                    (h || []).forEach((m) => {
                        const key = m.peer;
                        if (!key) return;
                        (c[key] = c[key] || []).push({
                            id: ++seq.current,
                            dir: m.dir === 'out' ? 'out' : 'in',
                            kind: m.kind || 'text',
                            text: m.text || '',
                            files: m.files || [],
                            time: m.time || '',
                        });
                    });
                    setConvs(c);
                })
                .catch(() => undefined)
                .finally(() => {
                    historyReady.current = true;
                });
        })();
    }, []);

    function addToast(title: string, body: string) {
        const id = ++seq.current;
        setToasts((t) => [...t, {id, title, body}]);
        window.setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4500);
    }

    function addConv(key: string, msg: Omit<ChatMsg, 'id'>) {
        setConvs((c) => ({...c, [key]: [...(c[key] || []), {...msg, id: ++seq.current}]}));
    }

    function triggerShake() {
        setShake(true);
        window.setTimeout(() => setShake(false), 600);
    }

    useEffect(() => {
        const offMembers = onEvent('members', (m: Member[]) => setMembers(m || []));
        const offMessage = onEvent('message', (m: any) => {
            const peer: Member = m.peer || {name: '', ip: '', port: 0};
            const key = `${peer.ip}:${peer.port}`;
            const kind = m.kind || 'text';
            if (kind === 'input') {
                setTyping((t) => ({...t, [key]: true}));
                return;
            }
            if (kind === 'inputend') {
                setTyping((t) => ({...t, [key]: false}));
                return;
            }
            if (kind === 'open') {
                return;
            }
            const files = (m.files || []).map((f: any) => f.Name);
            addConv(key, {dir: 'in', kind, text: m.text || '', files, time: clock()});
            setTyping((t) => ({...t, [key]: false}));

            if (kind === 'knock') {
                triggerShake();
                addToast(`🪟 ${peer.name || peer.ip}`, '戳了你一下');
                beep();
                return;
            }
            const active = selectedRef.current === key && focusRef.current;
            if (!active) {
                setUnread((u) => ({...u, [key]: (u[key] || 0) + 1}));
                const body = kind === 'image' ? '[图片]' : m.text || '[新消息]';
                addToast(peer.name || peer.ip, body.slice(0, 80));
                beep();
                sysNotify(peer.name || 'FlyDog', body.slice(0, 120));
            }
        });
        const offTransfer = onEvent('transfer', (t: Transfer) =>
            setTransfers((prev) => ({...prev, [t.id]: t})),
        );
        const offError = onEvent('engine:error', (s: string) => setError(s));
        return () => {
            offMembers();
            offMessage();
            offTransfer();
            offError();
        };
    }, []);

    // Persist chat history (debounced) whenever conversations change.
    useEffect(() => {
        if (!historyReady.current) return;
        window.clearTimeout(saveTimer.current);
        saveTimer.current = window.setTimeout(() => {
            const flat: any[] = [];
            Object.keys(convs).forEach((key) =>
                (convs[key] || []).forEach((m) =>
                    flat.push({peer: key, dir: m.dir, kind: m.kind, text: m.text, files: m.files, time: m.time}),
                ),
            );
            SaveHistory(flat).catch(() => undefined);
        }, 600);
        return () => window.clearTimeout(saveTimer.current);
    }, [convs]);

    const totalUnread = Object.values(unread).reduce((a, b) => a + b, 0);

    useEffect(() => {
        document.title = (totalUnread ? `(${totalUnread}) ` : '') + 'FlyDog';
    }, [totalUnread]);

    const selected = useMemo(
        () => members.find((m) => peerKey(m) === selectedKey) || null,
        [members, selectedKey],
    );

    useEffect(() => {
        bottomRef.current?.scrollIntoView({behavior: 'smooth'});
    }, [convs, selectedKey, typing]);

    const filtered = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return members;
        return members.filter(
            (m) => m.name.toLowerCase().includes(q) || m.host.toLowerCase().includes(q) || m.ip.includes(q),
        );
    }, [members, search]);

    const transferList = Object.values(transfers).slice(-30).reverse();

    async function toggle() {
        setError('');
        if (running) {
            await Stop();
            setRunning(false);
            setMembers([]);
            return;
        }
        try {
            if ('Notification' in window && Notification.permission === 'default') {
                Notification.requestPermission();
            }
            persist(name, downloadDir, autoReceive, myAvatar, trayMode);
            await Start(name, downloadDir, autoReceive);
            setRunning(true);
        } catch (e: any) {
            setError(String(e));
        }
    }

    function selectPeer(m: Member) {
        const key = peerKey(m);
        setSelectedKey(key);
        setUnread((u) => ({...u, [key]: 0}));
        setTyping((t) => ({...t, [key]: false}));
        OpenYou(m.ip, m.port).catch(() => undefined);
    }

    async function send() {
        const m = selected;
        if (!m || !input.trim()) return;
        const text = input;
        setInput('');
        window.clearTimeout(typingTimer.current);
        SendTyping(m.ip, m.port, false).catch(() => undefined);
        try {
            await SendText(m.ip, m.port, text);
            addConv(peerKey(m), {dir: 'out', kind: 'text', text, files: [], time: clock()});
        } catch (e: any) {
            setError(String(e));
        }
    }

    function onInputChange(v: string) {
        setInput(v);
        const m = selected;
        if (!m) return;
        const now = Date.now();
        if (now - lastTyping.current > 2000) {
            lastTyping.current = now;
            SendTyping(m.ip, m.port, true).catch(() => undefined);
        }
        window.clearTimeout(typingTimer.current);
        typingTimer.current = window.setTimeout(() => {
            SendTyping(m.ip, m.port, false).catch(() => undefined);
        }, 3000);
    }

    async function knock() {
        const m = selected;
        if (!m) return;
        try {
            await SendKnock(m.ip, m.port);
        } catch (e: any) {
            setError(String(e));
        }
        triggerShake();
    }

    async function attach(kind: 'file' | 'folder') {
        const m = selected;
        if (!m) return;
        try {
            const paths = kind === 'file' ? await PickFiles() : [await PickFolder()];
            const valid = (paths || []).filter((p) => p);
            if (valid.length === 0) return;
            const files = await SendFiles(m.ip, m.port, input, valid);
            setInput('');
            addConv(peerKey(m), {
                dir: 'out',
                kind: 'text',
                text: '',
                files: (files || []).map((f: any) => f.Name),
                time: clock(),
            });
        } catch (e: any) {
            setError(String(e));
        }
    }

    function persist(n: string, dir: string, auto: boolean, avatar: string, tray: boolean) {
        SaveConfig({
            ...cfgRef.current,
            name: n,
            downloadDir: dir,
            autoReceive: auto,
            avatar,
            minimizeToTray: tray,
        } as any).catch(() => undefined);
    }

    function saveCfg(overrides: any) {
        SaveConfig({...cfgRef.current, ...overrides} as any).catch(() => undefined);
    }

    function applyName() {
        const v = name.trim();
        if (!v) return;
        SetName(v).catch(() => undefined);
        persist(v, downloadDir, autoReceive, myAvatar, trayMode);
    }

    function chooseAvatar(v: string) {
        setMyAvatar(v);
        persist(name, downloadDir, autoReceive, v, trayMode);
        setShowAvatars(false);
    }

    function refreshLastRuns() {
        NightOffLastRuns()
            .then((r: any) => r && setLastRuns(r))
            .catch(() => undefined);
    }

    async function applyNightOff() {
        if (!nightOffAdapter) {
            setError('请先选择要断开的网卡');
            return;
        }
        setNightOffBusy(true);
        setError('');
        try {
            await InstallNightOff(Number(nightOffAdapter), nightOffStart, nightOffEnd);
            saveCfg({nightOffEnabled: true, nightOffAdapter, nightOffStart, nightOffEnd});
            setNightOffEnabled(true);
            setNightOffInstalled(await NightOffStatus());
        } catch (e: any) {
            setError(String(e));
        } finally {
            setNightOffBusy(false);
            refreshLastRuns();
        }
    }

    async function removeNightOff() {
        setNightOffBusy(true);
        setError('');
        try {
            await RemoveNightOff();
            saveCfg({nightOffEnabled: false});
            setNightOffEnabled(false);
            setNightOffInstalled(await NightOffStatus());
        } catch (e: any) {
            setError(String(e));
        } finally {
            setNightOffBusy(false);
            refreshLastRuns();
        }
    }

    async function nightOffNow(enable: boolean) {
        if (!nightOffAdapter) {
            setError('请先选择网卡');
            return;
        }
        if (!enable && !window.confirm(`确定现在禁用网卡「${nightOffAdapterName}」？\n禁用后将暂时断网。`)) {
            return;
        }
        setNightOffBusy(true);
        setError('');
        try {
            await NightOffNow(Number(nightOffAdapter), enable);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setNightOffBusy(false);
            refreshLastRuns();
        }
    }

    function uploadAvatar(file: File) {
        const r = new FileReader();
        r.onload = () => {
            if (typeof r.result === 'string') chooseAvatar(r.result);
        };
        r.readAsDataURL(file);
    }

    async function refresh() {
        if (!running) return;
        try {
            await Rescan();
        } catch (e: any) {
            setError(String(e));
        }
        setTimeout(() => GetMembers().then((m: any) => setMembers(m || [])), 1200);
        setTimeout(() => GetMembers().then((m: any) => setMembers(m || [])), 3000);
    }

    async function doScan() {
        if (!running) {
            setError('尚未启动：请先点右上角「启动」。');
            return;
        }
        try {
            await Scan(scanTarget.trim());
            setError('');
            [1500, 4000, 8000].forEach((t) => setTimeout(() => GetMembers().then((m: any) => setMembers(m || [])), t));
        } catch (e: any) {
            setError(String(e));
        }
    }

    async function addPeer() {
        const ip = peerIp.trim();
        if (!ip) return;
        try {
            await AddPeer(ip);
            setPeerIp('');
            [800, 2000].forEach((t) => setTimeout(() => GetMembers().then((m: any) => setMembers(m || [])), t));
        } catch (e: any) {
            setError(String(e));
        }
    }

    const chat = selectedKey ? convs[selectedKey] || [] : [];
    const nightOffAdapterName =
        adapters.find((a) => String(a.index) === nightOffAdapter)?.name || nightOffAdapter || '未选择';

    return (
        <div className={'app' + (shake ? ' shake' : '')}>
            <aside className="sidebar">
                <div className="me">
                    <button className="me-avatar-btn" title="点击更换头像" onClick={() => setShowAvatars(true)}>
                        <Avatar src={myAvatar} name={name} size={42} />
                    </button>
                    <div className="me-info">
                        <input
                            className="me-name-input"
                            value={name}
                            title="点击修改昵称，回车生效"
                            onChange={(e) => setName(e.target.value)}
                            onBlur={applyName}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                            }}
                        />
                        <div className="me-status">
                            <span className={running ? 'dot on' : 'dot'} />
                            {running ? '在线' : '未启动'}
                        </div>
                    </div>
                    <button
                        className={'power' + (running ? ' on' : '')}
                        title={running ? '点击停止' : '点击启动'}
                        onClick={toggle}
                    >
                        {running ? '■' : '▶'}
                    </button>
                    <button
                        className="icon-btn"
                        title={theme === 'dark' ? '切换到浅色' : '切换到深色'}
                        onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                    >
                        {theme === 'dark' ? '☀' : '🌙'}
                    </button>
                    <button className="icon-btn" title="设置" onClick={() => setShowSettings(true)}>⚙</button>
                </div>

                <div className="search">
                    <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="搜索用户 / IP" />
                </div>

                <div className="list">
                    {filtered.length === 0 && <div className="empty">{running ? '未发现在线用户' : '点击「启动」开始'}</div>}
                    {filtered.map((m) => {
                        const key = peerKey(m);
                        const n = unread[key] || 0;
                        return (
                            <div key={key} className={'contact' + (key === selectedKey ? ' active' : '')} onClick={() => selectPeer(m)}>
                                <div className="avatar-wrap">
                                    <Avatar name={m.name} size={38} />
                                    <span className={'presence' + (m.absence ? ' off' : '')} />
                                </div>
                                <div className="contact-info">
                                    <div className="contact-name">{m.name || m.ip}</div>
                                    <div className="contact-sub">
                                        {m.ip}
                                        {m.group ? ` · ${m.group}` : ''}
                                        {m.host && m.host !== m.name ? ` · ${m.host}` : ''}
                                        {m.absence ? ' · 离开' : ''}
                                    </div>
                                </div>
                                {typing[key] && <span className="typing-tag">输入中</span>}
                                {n > 0 && <span className="badge">{n > 99 ? '99+' : n}</span>}
                            </div>
                        );
                    })}
                </div>

                <div className="sidebar-actions">
                    <button onClick={refresh} disabled={!running}>刷新在线用户</button>
                </div>
            </aside>

            <main className="chat">
                {selected ? (
                    <>
                        <header className="chat-head">
                            <div className="avatar-wrap">
                                <Avatar name={selected.name} size={36} />
                                <span className={'presence' + (selected.absence ? ' off' : '')} />
                            </div>
                            <div className="chat-title">
                                <div className="chat-name">{selected.name}</div>
                                <div className={'chat-sub' + (typing[selectedKey] ? ' typing' : '')}>
                                    {typing[selectedKey] ? '正在输入…' : `${selected.ip}${selected.group ? ' · ' + selected.group : ''}${selected.absence ? ' · 离开' : ''}`}
                                </div>
                            </div>
                            <div className="chat-actions">
                                <button className="ghost" title="窗口抖动" onClick={knock}>🪟 抖一抖</button>
                                <button className="ghost" onClick={() => attach('file')}>文件</button>
                                <button className="ghost" onClick={() => attach('folder')}>文件夹</button>
                            </div>
                        </header>

                        <div className="messages">
                            {chat.map((msg, i) => {
                                const prev = chat[i - 1];
                                const grouped =
                                    !!prev &&
                                    prev.dir === msg.dir &&
                                    prev.kind === msg.kind &&
                                    (prev.text !== '') === (msg.text !== '');
                                return (
                                    <div key={msg.id} className={'bubble-row ' + msg.dir + (grouped ? ' grouped' : '')}>
                                        {msg.dir === 'in' &&
                                            (grouped ? <div className="avatar-spacer" /> : <Avatar name={selected.name} size={30} />)}
                                        <div className="bubble-wrap">
                                            {msg.kind === 'knock' ? (
                                                <div className="knock-msg">🪟 窗口抖动</div>
                                            ) : msg.kind === 'image' ? (
                                                <div className="bubble">[图片]</div>
                                            ) : (
                                                <div className="bubble">
                                                    {msg.text && <div className="text">{msg.text}</div>}
                                                    {msg.files.map((f, j) => (
                                                        <div className="file" key={j}>📎 {f}</div>
                                                    ))}
                                                </div>
                                            )}
                                            <div className="stamp">{msg.time}</div>
                                        </div>
                                        {msg.dir === 'out' &&
                                            (grouped ? <div className="avatar-spacer" /> : <Avatar src={myAvatar} name={name} size={30} />)}
                                    </div>
                                );
                            })}
                            {typing[selectedKey] && (
                                <div className="bubble-row in">
                                    <Avatar name={selected.name} size={30} />
                                    <div className="bubble typing-bubble">
                                        <span className="tdot" /><span className="tdot" /><span className="tdot" />
                                    </div>
                                </div>
                            )}
                            <div ref={bottomRef} />
                        </div>

                        <footer className="composer">
                            <div className="composer-box">
                                <div className="composer-tools">
                                    <button className="icon-btn" title="表情" onClick={() => setEmojiOpen((v) => !v)}>😊</button>
                                    <button className="icon-btn" title="发送文件" onClick={() => attach('file')}>📎</button>
                                    <button className="icon-btn" title="发送文件夹" onClick={() => attach('folder')}>📁</button>
                                </div>
                                <textarea
                                    className="msg-input"
                                    value={input}
                                    onChange={(e) => onInputChange(e.target.value)}
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter' && !e.shiftKey) {
                                            e.preventDefault();
                                            send();
                                        }
                                    }}
                                    placeholder="输入消息，Enter 发送，Shift+Enter 换行"
                                    rows={2}
                                />
                            </div>
                            <button className="send-btn" onClick={send} disabled={!input.trim()}>发送</button>
                            {emojiOpen && (
                                <div className="emoji-pop">
                                    {EMOJIS.map((e) => (
                                        <button key={e} onClick={() => { setInput((v) => v + e); setEmojiOpen(false); }}>{e}</button>
                                    ))}
                                </div>
                            )}
                        </footer>
                    </>
                ) : (
                    <div className="welcome">
                        <div className="welcome-logo">🐕</div>
                        <div className="welcome-title">FlyDog 局域网传输</div>
                        <div className="welcome-sub">{running ? '从左侧选择一位用户开始聊天' : '点击右上角「启动」开始使用'}</div>
                    </div>
                )}
            </main>

            <aside className={'transfers' + (showTransfers ? '' : ' collapsed')}>
                {showTransfers ? (
                    <>
                        <div className="panel-head">
                            <span>传输</span>
                            <div className="panel-head-actions">
                                <button className="link" onClick={() => OpenDownloadDir(downloadDir)}>目录</button>
                                <button className="link" onClick={() => setShowTransfers(false)}>收起 ›</button>
                            </div>
                        </div>
                        <div className="transfer-list">
                            {transferList.length === 0 && <div className="empty">暂无传输</div>}
                            {transferList.map((t) => {
                                const pct = t.size > 0 ? Math.min(100, Math.round((t.done / t.size) * 100)) : t.state === 'done' ? 100 : 0;
                                return (
                                    <div className="transfer" key={t.id}>
                                        <div className="t-row">
                                            <span className="t-dir">{t.direction === 'send' ? '↑' : '↓'}</span>
                                            <span className="t-name" title={t.name}>{t.name}</span>
                                            <span className={'t-state ' + t.state}>{t.state}</span>
                                        </div>
                                        <div className="bar"><div className="fill" style={{width: pct + '%'}} /></div>
                                        <div className="t-sub">
                                            {t.size > 0 ? `${fmtSize(t.done)} / ${fmtSize(t.size)}` : fmtSize(t.done)}
                                            {t.error ? ` · ${t.error}` : ''}
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    </>
                ) : (
                    <button className="rail-btn" title="展开传输面板" onClick={() => setShowTransfers(true)}>
                        <span className="rail-arrow">‹</span>
                        <span className="rail-label">传输</span>
                    </button>
                )}
            </aside>

            {error && <div className="error-bar" onClick={() => setError('')}>⚠ {error}</div>}

            <div className="toasts">
                {toasts.map((t) => (
                    <div className="toast" key={t.id} onClick={() => setToasts((x) => x.filter((y) => y.id !== t.id))}>
                        <div className="toast-title">{t.title}</div>
                        <div className="toast-body">{t.body}</div>
                    </div>
                ))}
            </div>

            {showAvatars && (
                <div className="modal-mask" onClick={() => setShowAvatars(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-head">
                            <span>选择头像</span>
                            <button className="icon-btn" onClick={() => setShowAvatars(false)}>✕</button>
                        </div>
                        <div className="modal-body">
                            <div className="avatar-grid">
                                {Array.from({length: AVATAR_COUNT}, (_, i) => (
                                    <button
                                        key={i}
                                        className={'avatar-cell' + (myAvatar === 'builtin:' + i ? ' active' : '')}
                                        onClick={() => chooseAvatar('builtin:' + i)}
                                        title={'头像 ' + (i + 1)}
                                    >
                                        <BuiltinAvatar index={i} size={64} />
                                    </button>
                                ))}
                            </div>
                            <div className="divider">自定义</div>
                            <div className="inline">
                                <button className="btn" onClick={() => chooseAvatar('builtin:' + Math.floor(Math.random() * AVATAR_COUNT))}>
                                    随机一个
                                </button>
                                <label className="btn file-btn">
                                    上传图片
                                    <input
                                        type="file"
                                        accept="image/*"
                                        style={{display: 'none'}}
                                        onChange={(e) => {
                                            const f = e.target.files?.[0];
                                            if (f) uploadAvatar(f);
                                            e.target.value = '';
                                        }}
                                    />
                                </label>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {showSettings && (
                <div className="modal-mask" onClick={() => setShowSettings(false)}>
                    <div className="modal settings" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-head">
                            <span>设置</span>
                            <button className="icon-btn" onClick={() => setShowSettings(false)}>✕</button>
                        </div>
                        <div className="settings-layout">
                            <nav className="settings-nav">
                                {SETTINGS_TABS.map((t) => (
                                    <button
                                        key={t.key}
                                        className={'settings-tab' + (settingsTab === t.key ? ' active' : '')}
                                        onClick={() => setSettingsTab(t.key)}
                                    >
                                        <span className="tab-ico">{t.icon}</span>
                                        {t.label}
                                    </button>
                                ))}
                            </nav>
                            <div className="settings-content">
                                {settingsTab === 'profile' && (
                                    <section className="pane">
                                        <h4>个人资料</h4>
                                        <label className="field">
                                            <span>昵称</span>
                                            <input
                                                value={name}
                                                onChange={(e) => setName(e.target.value)}
                                                onBlur={applyName}
                                                onKeyDown={(e) => {
                                                    if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                                                }}
                                            />
                                        </label>
                                        <div className="field row">
                                            <Avatar src={myAvatar} name={name} size={44} />
                                            <button className="btn" onClick={() => setShowAvatars(true)}>更换头像</button>
                                        </div>
                                        <p className="hint">修改昵称会自动广播给局域网内其他人；其他人的头像默认随机生成。</p>
                                    </section>
                                )}

                                {settingsTab === 'files' && (
                                    <section className="pane">
                                        <h4>文件</h4>
                                        <label className="field">
                                            <span>接收目录（下次启动生效）</span>
                                            <input
                                                value={downloadDir}
                                                onChange={(e) => {
                                                    setDownloadDir(e.target.value);
                                                    persist(name, e.target.value, autoReceive, myAvatar, trayMode);
                                                }}
                                            />
                                        </label>
                                        <label className="field row">
                                            <input
                                                type="checkbox"
                                                checked={autoReceive}
                                                onChange={(e) => {
                                                    setAutoReceive(e.target.checked);
                                                    persist(name, downloadDir, e.target.checked, myAvatar, trayMode);
                                                }}
                                            />
                                            <span>自动接收文件</span>
                                        </label>

                                        <div className="divider">聊天记录</div>
                                        <div className="field row">
                                            <button
                                                className="btn"
                                                onClick={() => {
                                                    setConvs({});
                                                    ClearHistory().catch(() => undefined);
                                                }}
                                            >
                                                清空聊天记录
                                            </button>
                                            <span className="muted">保存在本机</span>
                                        </div>
                                    </section>
                                )}

                                {settingsTab === 'network' && (
                                    <>
                                        <section className="pane">
                                            <h4>发现用户</h4>
                                            <div className="inline">
                                                <input
                                                    value={scanTarget}
                                                    onChange={(e) => setScanTarget(e.target.value)}
                                                    placeholder="扫描网段 如 172.17.32. 或 /24"
                                                    disabled={!running}
                                                />
                                                <button className="btn" onClick={doScan} disabled={!running}>扫描</button>
                                            </div>
                                            <div className="inline">
                                                <input
                                                    value={peerIp}
                                                    onChange={(e) => setPeerIp(e.target.value)}
                                                    placeholder="单机直连 IP"
                                                    disabled={!running}
                                                />
                                                <button className="btn" onClick={addPeer} disabled={!running || !peerIp.trim()}>直连</button>
                                            </div>
                                            <p className="hint">同一 VLAN 会自动发现；跨网段时用扫描或直连。</p>
                                        </section>

                                        <section className="pane">
                                            <h4>夜间断网</h4>
                                            <label className="field row">
                                                <input
                                                    type="checkbox"
                                                    checked={nightOffEnabled}
                                                    onChange={(e) => {
                                                        setNightOffEnabled(e.target.checked);
                                                        saveCfg({nightOffEnabled: e.target.checked});
                                                    }}
                                                />
                                                <span>启用（到点自动禁用网卡，次日恢复）</span>
                                            </label>
                                            <label className="field">
                                                <span>网卡</span>
                                                <select
                                                    className="select"
                                                    value={nightOffAdapter}
                                                    onChange={(e) => {
                                                        setNightOffAdapter(e.target.value);
                                                        saveCfg({nightOffAdapter: e.target.value});
                                                    }}
                                                >
                                                    <option value="">请选择网卡</option>
                                                    {adapters.map((ad) => (
                                                        <option key={ad.index} value={String(ad.index)}>
                                                            {ad.name} — {ad.desc}（{ad.status}）
                                                        </option>
                                                    ))}
                                                </select>
                                            </label>
                                            <div className="inline">
                                                <label className="field" style={{flex: 1}}>
                                                    <span>断网时间</span>
                                                    <input
                                                        type="time"
                                                        value={nightOffStart}
                                                        onChange={(e) => {
                                                            setNightOffStart(e.target.value);
                                                            saveCfg({nightOffStart: e.target.value});
                                                        }}
                                                    />
                                                </label>
                                                <label className="field" style={{flex: 1}}>
                                                    <span>恢复时间</span>
                                                    <input
                                                        type="time"
                                                        value={nightOffEnd}
                                                        onChange={(e) => {
                                                            setNightOffEnd(e.target.value);
                                                            saveCfg({nightOffEnd: e.target.value});
                                                        }}
                                                    />
                                                </label>
                                            </div>
                                            <div className="inline">
                                                <button className="btn primary" disabled={nightOffBusy || !nightOffAdapter} onClick={applyNightOff}>
                                                    {nightOffBusy ? '处理中…' : nightOffInstalled ? '更新定时任务' : '应用（需要管理员）'}
                                                </button>
                                                <button className="btn" disabled={nightOffBusy || !nightOffInstalled} onClick={removeNightOff}>
                                                    取消定时任务
                                                </button>
                                            </div>
                                            <div className="inline">
                                                <button className="btn" disabled={nightOffBusy || !nightOffAdapter} onClick={() => nightOffNow(false)}>
                                                    立即断网
                                                </button>
                                                <button className="btn" disabled={nightOffBusy || !nightOffAdapter} onClick={() => nightOffNow(true)}>
                                                    立即恢复
                                                </button>
                                                <span className="muted" style={{alignSelf: 'center'}}>测试用，会弹 UAC</span>
                                            </div>
                                            <p className="hint">
                                                {nightOffInstalled
                                                    ? `已生效：每天 ${nightOffStart} 禁用「${nightOffAdapterName}」，${nightOffEnd} 恢复（由 Windows 计划任务执行，FlyDog 关闭也会生效）`
                                                    : '未安装。点「应用」会弹出 UAC，请允许管理员权限。'}
                                            </p>
                                            <p className="hint">
                                                {lastRuns.disableTime || lastRuns.enableTime
                                                    ? `最近执行：断网 ${lastRuns.disableTime || '—'} → ${lastRuns.disableStatus || '—'}　｜　恢复 ${lastRuns.enableTime || '—'} → ${lastRuns.enableStatus || '—'}`
                                                    : '最近没有执行记录'}
                                            </p>
                                        </section>
                                    </>
                                )}

                                {settingsTab === 'system' && (
                                    <section className="pane">
                                        <h4>启动与运行</h4>
                                        <label className="field row">
                                            <input
                                                type="checkbox"
                                                checked={trayMode}
                                                onChange={(e) => {
                                                    setTrayMode(e.target.checked);
                                                    persist(name, downloadDir, autoReceive, myAvatar, e.target.checked);
                                                }}
                                            />
                                            <span>关闭窗口时最小化到托盘（右键托盘图标可退出）</span>
                                        </label>
                                        <label className="field row">
                                            <input
                                                type="checkbox"
                                                checked={autostart}
                                                onChange={(e) => {
                                                    const v = e.target.checked;
                                                    setAutostart(v);
                                                    SetAutostart(v).catch((err: any) => {
                                                        setAutostart(!v);
                                                        setError(String(err));
                                                    });
                                                }}
                                            />
                                            <span>开机自动启动（启动后留在托盘）</span>
                                        </label>

                                        <div className="divider">服务状态</div>
                                        <div className="field row">
                                            <span className={running ? 'dot on' : 'dot'} />
                                            <span>{running ? '正在运行' : '已停止'}</span>
                                            <button className={running ? 'btn stop' : 'btn primary'} onClick={toggle}>
                                                {running ? '停止' : '启动'}
                                            </button>
                                        </div>
                                    </section>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export default App;
