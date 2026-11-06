import {Fragment, useEffect, useMemo, useRef, useState, type ReactNode} from 'react';
import './App.css';
import {Events} from '@wailsio/runtime';
import {App as AppSvc} from '../bindings/neiwangtong';

const {
    AddPeer,
    ClearHistory,
    CopyText,
    GetAutostart,
    GetMembers,
    GetShareInfo,
    InstallNightOff,
    IsRunning,
    ListAdapters,
    LoadConfig,
    LoadHistory,
    NightOffLastRuns,
    NightOffNow,
    NightOffStatus,
    OpenDownloadDir,
    OpenURL,
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
    StartShare,
    Stop,
    StopShare,
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

interface Contact extends Member {
    offline?: boolean;
    lastText?: string;
    lastAt?: string;
}

interface ChatMsg {
    id: number;
    dir: 'in' | 'out';
    kind: string;
    text: string;
    files: string[];
    time: string;
    peerName?: string;
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

interface ShareInfo {
    running: boolean;
    dir: string;
    port: number;
    allowUpload: boolean;
    urls: string[];
}

interface Toast {
    id: number;
    title: string;
    body: string;
}

const SETTINGS_TABS: {key: string; icon: IconName; label: string}[] = [
    {key: 'profile', icon: 'user', label: '个人'},
    {key: 'files', icon: 'folder', label: '文件与记录'},
    {key: 'share', icon: 'share', label: '共享'},
    {key: 'network', icon: 'network', label: '网络'},
    {key: 'system', icon: 'settings', label: '系统'},
];

const EMOJIS = [
    '😀', '😁', '😂', '🤣', '😊', '😍', '😘', '😜', '🤔', '😎', '😭', '😡',
    '👍', '👎', '👌', '🙏', '👏', '💪', '🎉', '🔥', '💯', '❤️', '💔', '🌹',
    '🍀', '☕', '🍺', '🎁', '⭐', '✅', '❌', '⚠️', '😴', '🤝', '🤗', '😇',
    '🥳', '😢', '😅', '🙄', '🚀', '🐶', '📎', '📁', '🖥️', '📱', '⏰', '🎵',
];

type IconName =
    | 'power'
    | 'sun'
    | 'moon'
    | 'settings'
    | 'refresh'
    | 'bell'
    | 'user'
    | 'folder'
    | 'share'
    | 'network'
    | 'chevronLeft';

const ICON_PATHS: Record<IconName, ReactNode> = {
    power: (
        <>
            <path d="M12 3v9" />
            <path d="M6.6 6.6a8 8 0 1 0 10.8 0" />
        </>
    ),
    sun: (
        <>
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
        </>
    ),
    moon: <path d="M20.5 14.3A8.5 8.5 0 0 1 9.7 3.5a8.5 8.5 0 1 0 10.8 10.8Z" />,
    settings: (
        <>
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1 1.55V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1-1.55 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-1.55-1H3a2 2 0 1 1 0-4h.09A1.7 1.7 0 0 0 4.6 9a1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.55V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1 1.55 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.7 1.7 0 0 0 19.4 9v.09a1.7 1.7 0 0 0 1.55 1H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.51 1Z" />
        </>
    ),
    refresh: (
        <>
            <path d="M20.5 12a8.5 8.5 0 1 1-2.5-6" />
            <path d="M20.5 3.5V9H15" />
        </>
    ),
    bell: (
        <>
            <path d="M18 8.5a6 6 0 0 0-12 0c0 6.5-2.5 8.5-2.5 8.5h17s-2.5-2-2.5-8.5Z" />
            <path d="M13.7 20.5a2 2 0 0 1-3.4 0" />
        </>
    ),
    user: (
        <>
            <circle cx="12" cy="8" r="3.6" />
            <path d="M4.8 20.5a7.2 7.2 0 0 1 14.4 0" />
        </>
    ),
    folder: <path d="M3.5 7.5a2 2 0 0 1 2-2h4.4l2 2h6.6a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2Z" />,
    share: (
        <>
            <circle cx="6" cy="12" r="2.2" />
            <circle cx="18" cy="6" r="2.2" />
            <circle cx="18" cy="18" r="2.2" />
            <path d="m8 11 8-4M8 13l8 4" />
        </>
    ),
    network: (
        <>
            <circle cx="12" cy="5" r="2.2" />
            <circle cx="5" cy="19" r="2.2" />
            <circle cx="19" cy="19" r="2.2" />
            <path d="M12 7.2v4.3M12 11.5 6.5 17.2M12 11.5l5.5 5.7" />
        </>
    ),
    chevronLeft: <path d="m15 5-7 7 7 7" />,
};

// PawMark is the one piece of brand playfulness in the product: the app icon
// reduced to a single cobalt glyph.
function PawMark({size = 56}: {size?: number}) {
    return (
        <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
            <g fill="currentColor">
                <ellipse cx="32" cy="43" rx="12.5" ry="9.5" />
                <ellipse cx="17.5" cy="25" rx="4.8" ry="6.6" transform="rotate(-25 17.5 25)" />
                <ellipse cx="26.5" cy="19" rx="5" ry="7" transform="rotate(-8 26.5 19)" />
                <ellipse cx="37.5" cy="19" rx="5" ry="7" transform="rotate(8 37.5 19)" />
                <ellipse cx="46.5" cy="25" rx="4.8" ry="6.6" transform="rotate(25 46.5 25)" />
            </g>
        </svg>
    );
}

function Icon({name, size = 16}: {name: IconName; size?: number}) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.8}
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
        >
            {ICON_PATHS[name]}
        </svg>
    );
}

const DEFAULT_GROUP = '内网通联系人';

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

function pad2(n: number): string {
    return String(n).padStart(2, '0');
}

// Persist a full timestamp so history is still meaningful days later.
function stampNow(): string {
    return new Date().toISOString();
}

// fmtStamp renders a stored timestamp; older history entries only contain
// "HH:mm" and are shown as-is.
function fmtStamp(v: string): string {
    if (!v) return '';
    if (/^\d{1,2}:\d{2}$/.test(v)) return v;
    const d = new Date(v);
    if (Number.isNaN(d.getTime())) return v;
    const hm = `${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
    const today = new Date();
    if (d.toDateString() === today.toDateString()) return hm;
    const yesterday = new Date(today);
    yesterday.setDate(today.getDate() - 1);
    if (d.toDateString() === yesterday.toDateString()) return `昨天 ${hm}`;
    const md = `${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
    return d.getFullYear() === today.getFullYear() ? `${md} ${hm}` : `${d.getFullYear()}-${md} ${hm}`;
}

// peerKey is "ip:port"; IPv4 addresses contain no ':' so the last one splits it.
function parsePeerKey(key: string): {ip: string; port: number} {
    const i = key.lastIndexOf(':');
    if (i < 0) return {ip: key, port: 0};
    return {ip: key.slice(0, i), port: Number(key.slice(i + 1)) || 0};
}

// Turn http(s) URLs inside a message into clickable links; they open in the
// system browser rather than navigating the Wails webview.
function renderText(text: string): ReactNode {
    const parts = text.split(/(https?:\/\/[^\s]+)/g);
    return parts.map((part, i) =>
        /^https?:\/\//.test(part) ? (
            <a
                key={i}
                className="msg-link"
                href={part}
                onClick={(e) => {
                    e.preventDefault();
                    OpenURL(part).catch(() => undefined);
                }}
            >
                {part}
            </a>
        ) : (
            <span key={i}>{part}</span>
        ),
    );
}

// Chromium allows only a handful of AudioContexts per page, so creating one
// per beep silently stops the sound after ~6 notifications. Keep one instead.
let audioCtx: any = null;

function beep(): void {
    try {
        const Ctx = (window as any).AudioContext || (window as any).webkitAudioContext;
        if (!audioCtx) audioCtx = new Ctx();
        const ctx = audioCtx;
        if (ctx.state === 'suspended') ctx.resume();
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
    const [showTransfers, setShowTransfers] = useState(false);
    const [search, setSearch] = useState('');
    const [peerIp, setPeerIp] = useState('');
    const [scanTarget, setScanTarget] = useState('');
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

    // 共享文件夹
    const [shareDir, setShareDir] = useState('');
    const [sharePort, setSharePort] = useState(2427);
    const [shareAllowUpload, setShareAllowUpload] = useState(true);
    const [shareInfo, setShareInfo] = useState<ShareInfo | null>(null);
    const [shareBusy, setShareBusy] = useState(false);

    const [theme, setTheme] = useState<string>(() => {
        // Keep the theme chosen before the rename, then store it under the
        // new key from now on.
        return localStorage.getItem('neiwangtong.theme') || localStorage.getItem('flydog.theme') || 'light';
    });

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
    const seenTransfers = useRef<Set<string>>(new Set());

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
            shareDir,
            sharePort,
            shareAllowUpload,
        };
    }, [
        name,
        downloadDir,
        autoReceive,
        myAvatar,
        trayMode,
        nightOffEnabled,
        nightOffAdapter,
        nightOffStart,
        nightOffEnd,
        shareDir,
        sharePort,
        shareAllowUpload,
    ]);

    useEffect(() => {
        document.documentElement.dataset.theme = theme;
        localStorage.setItem('neiwangtong.theme', theme);
        localStorage.removeItem('flydog.theme');
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
                setShareDir(c.shareDir || '');
                setSharePort(c.sharePort || 2427);
                setShareAllowUpload(c.shareAllowUpload !== false);
                GetShareInfo()
                    .then((s: any) => {
                        if (s && s.running) {
                            setShareInfo(s);
                            setShareDir(s.dir || c.shareDir || '');
                            setSharePort(s.port || c.sharePort || 2427);
                            setShareAllowUpload(!!s.allowUpload);
                        }
                    })
                    .catch(() => undefined);
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
                        shareDir: c.shareDir,
                        sharePort: c.sharePort,
                        shareAllowUpload: c.shareAllowUpload,
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
                            peerName: m.peerName || '',
                        });
                    });
                    // Merge instead of replacing: a message may have arrived
                    // while the history file was being read.
                    setConvs((live) => {
                        const merged: Record<string, ChatMsg[]> = {...c};
                        Object.keys(live).forEach((key) => {
                            merged[key] = [...(c[key] || []), ...live[key]];
                        });
                        return merged;
                    });
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
            addConv(key, {
                dir: 'in',
                kind,
                text: m.text || '',
                files,
                time: stampNow(),
                peerName: peer.name || peer.ip,
            });
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
                sysNotify(peer.name || '内网通', body.slice(0, 120));
            }
        });
        const offTransfer = onEvent('transfer', (t: Transfer) => {
            // Open the panel for a new transfer, but don't force it back open
            // while the user is watching progress or has collapsed it.
            if (!seenTransfers.current.has(t.id)) {
                seenTransfers.current.add(t.id);
                setShowTransfers(true);
            }
            setTransfers((prev) => ({...prev, [t.id]: t}));
        });
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
                    flat.push({
                        peer: key,
                        peerName: m.peerName || '',
                        dir: m.dir,
                        kind: m.kind,
                        text: m.text,
                        files: m.files,
                        time: m.time,
                    }),
                ),
            );
            SaveHistory(flat).catch(() => undefined);
        }, 600);
        return () => window.clearTimeout(saveTimer.current);
    }, [convs]);

    const totalUnread = Object.values(unread).reduce((a, b) => a + b, 0);

    useEffect(() => {
        document.title = (totalUnread ? `(${totalUnread}) ` : '') + '内网通';
    }, [totalUnread]);

    // Conversations whose peer is no longer online still have saved history;
    // surface them in the sidebar instead of hiding them until the peer
    // reappears.
    const historyContacts = useMemo<Contact[]>(() => {
        const online = new Set(members.map(peerKey));
        const out: Contact[] = [];
        Object.keys(convs).forEach((key) => {
            if (online.has(key)) return;
            const msgs = convs[key] || [];
            if (msgs.length === 0) return;
            const last = msgs[msgs.length - 1];
            const {ip, port} = parsePeerKey(key);
            const preview =
                last.kind === 'knock'
                    ? '窗口抖动'
                    : last.text.replace(/\s+/g, ' ').trim() ||
                      (last.files.length ? `[文件] ${last.files.join('、')}` : '');
            out.push({
                name: last.peerName || ip,
                group: '',
                host: '',
                ip,
                port,
                absence: true,
                utf8: false,
                offline: true,
                lastText: preview.slice(0, 40),
                lastAt: last.time,
            });
        });
        out.sort((a, b) => (b.lastAt || '').localeCompare(a.lastAt || '') || a.name.localeCompare(b.name));
        return out;
    }, [convs, members]);

    const selected = useMemo(
        () =>
            members.find((m) => peerKey(m) === selectedKey) ||
            historyContacts.find((m) => peerKey(m) === selectedKey) ||
            null,
        [members, historyContacts, selectedKey],
    );
    const selectedOffline = !!selected && !members.some((m) => peerKey(m) === selectedKey);

    useEffect(() => {
        bottomRef.current?.scrollIntoView({behavior: 'smooth'});
    }, [convs, selectedKey, typing]);

    const filteredMembers = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return members;
        return members.filter(
            (m) =>
                m.name.toLowerCase().includes(q) ||
                m.host.toLowerCase().includes(q) ||
                m.ip.includes(q) ||
                (m.group || '').toLowerCase().includes(q),
        );
    }, [members, search]);

    const groupedMembers = useMemo(() => {
        const groups = new Map<string, Member[]>();
        filteredMembers.forEach((m) => {
            const name = (m.group || '').trim() || DEFAULT_GROUP;
            const list = groups.get(name) || [];
            list.push(m);
            groups.set(name, list);
        });
        const out = Array.from(groups.entries()).map(([name, list]) => ({name, members: list}));
        out.sort((a, b) => {
            if (a.name === DEFAULT_GROUP) return -1;
            if (b.name === DEFAULT_GROUP) return 1;
            return a.name.localeCompare(b.name);
        });
        return out;
    }, [filteredMembers]);
    const showGroupHeaders =
        groupedMembers.length > 1 ||
        (groupedMembers.length === 1 && groupedMembers[0].name !== DEFAULT_GROUP);

    const filteredHistory = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return historyContacts;
        return historyContacts.filter(
            (m) =>
                m.name.toLowerCase().includes(q) ||
                m.ip.includes(q) ||
                (m.lastText || '').toLowerCase().includes(q),
        );
    }, [historyContacts, search]);

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

    function selectPeer(m: Member, offline = false) {
        const key = peerKey(m);
        setSelectedKey(key);
        setUnread((u) => ({...u, [key]: 0}));
        setTyping((t) => ({...t, [key]: false}));
        if (!offline) OpenYou(m.ip, m.port).catch(() => undefined);
    }

    async function send() {
        const m = selected;
        if (!m || selectedOffline || !input.trim()) return;
        const text = input;
        setInput('');
        window.clearTimeout(typingTimer.current);
        SendTyping(m.ip, m.port, false).catch(() => undefined);
        try {
            await SendText(m.ip, m.port, text);
            addConv(peerKey(m), {
                dir: 'out',
                kind: 'text',
                text,
                files: [],
                time: stampNow(),
                peerName: m.name || m.ip,
            });
        } catch (e: any) {
            setError(String(e));
        }
    }

    function onInputChange(v: string) {
        setInput(v);
        const m = selected;
        if (!m || selectedOffline) return;
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
        if (!m || selectedOffline) return;
        try {
            await SendKnock(m.ip, m.port);
        } catch (e: any) {
            setError(String(e));
        }
        triggerShake();
    }

    async function attach(kind: 'file' | 'folder') {
        const m = selected;
        if (!m || selectedOffline) return;
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
                time: stampNow(),
                peerName: m.name || m.ip,
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
        if (file.size > 512 * 1024) {
            setError('头像图片请小于 512KB');
            return;
        }
        const r = new FileReader();
        r.onload = () => {
            if (typeof r.result === 'string') chooseAvatar(r.result);
        };
        r.readAsDataURL(file);
    }

    async function pickShareDir() {
        try {
            const p = await PickFolder();
            if (p) setShareDir(p);
        } catch (e: any) {
            setError(String(e));
        }
    }

    async function startShare() {
        const dir = shareDir.trim();
        if (!dir) {
            setError('请先选择要共享的文件夹');
            return;
        }
        setShareBusy(true);
        setError('');
        try {
            const info: any = await StartShare(dir, Number(sharePort) || 2427, shareAllowUpload);
            setShareInfo(info);
            setSharePort(info.port || 2427);
            saveCfg({shareDir: dir, sharePort: info.port || 2427, shareAllowUpload});
            addToast('共享已开启', (info.urls && info.urls[0]) || '');
        } catch (e: any) {
            setError(String(e));
        } finally {
            setShareBusy(false);
        }
    }

    async function stopShare() {
        setShareBusy(true);
        setError('');
        try {
            await StopShare();
            setShareInfo(null);
            saveCfg({shareDir, sharePort, shareAllowUpload});
        } catch (e: any) {
            setError(String(e));
        } finally {
            setShareBusy(false);
        }
    }

    async function copyText(v: string) {
        try {
            await navigator.clipboard.writeText(v);
            addToast('已复制', v);
        } catch {
            try {
                await CopyText(v);
                addToast('已复制', v);
            } catch {
                setError('复制失败，请手动选择链接');
            }
        }
    }

    // Prefer the share URL on the same /24 as the peer, so cross-subnet setups
    // still hand out a reachable address.
    function pickShareURL(peerIP: string): string {
        const urls = shareInfo?.urls || [];
        if (!urls.length) return '';
        const prefix = peerIP.split('.').slice(0, 3).join('.') + '.';
        const match = urls.find((u) => {
            try {
                return new URL(u).hostname.startsWith(prefix);
            } catch {
                return false;
            }
        });
        return match || urls[0];
    }

    async function sendShareLink() {
        const m = selected;
        if (!m || selectedOffline) return;
        if (!shareInfo?.running) {
            setSettingsTab('share');
            setShowSettings(true);
            setError('还没有开启共享文件夹，请先在「设置 → 共享」里选择文件夹并开始共享。');
            return;
        }
        const url = pickShareURL(m.ip);
        if (!url) {
            setError('共享链接为空，请重新开始共享');
            return;
        }
        const text = `我给你共享了一个文件夹，打开链接可以浏览、下载和上传：\n${url}`;
        try {
            await SendText(m.ip, m.port, text);
            addConv(peerKey(m), {
                dir: 'out',
                kind: 'text',
                text,
                files: [],
                time: stampNow(),
                peerName: m.name || m.ip,
            });
            addToast('共享链接已发送', url);
        } catch (e: any) {
            setError(String(e));
        }
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

    function contactRow(m: Member) {
        const key = peerKey(m);
        const n = unread[key] || 0;
        return (
            <div
                key={key}
                className={'contact' + (key === selectedKey ? ' active' : '')}
                title={`${m.name || m.ip}\n${m.ip}${m.host ? '  ' + m.host : ''}`}
                onClick={() => selectPeer(m)}
            >
                <div className="avatar-wrap">
                    <Avatar name={m.name} size={38} />
                    <span className={'presence' + (m.absence ? ' off' : '')} />
                </div>
                <div className="contact-info">
                    <div className="contact-name">{m.name || m.ip}</div>
                    <div className="contact-sub">
                        <span className="mono">{m.ip}</span>
                        {m.absence && <span className="tag">离开</span>}
                    </div>
                </div>
                {typing[key] && <span className="typing-tag">输入中</span>}
                {n > 0 && <span className="badge">{n > 99 ? '99+' : n}</span>}
            </div>
        );
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
                        aria-label={running ? '停止服务' : '启动服务'}
                        onClick={toggle}
                    >
                        <Icon name="power" size={15} />
                    </button>
                    <button
                        className="icon-btn"
                        title={theme === 'dark' ? '切换到浅色' : '切换到深色'}
                        aria-label="切换主题"
                        onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                    >
                        <Icon name={theme === 'dark' ? 'sun' : 'moon'} size={16} />
                    </button>
                    <button className="icon-btn" title="设置" aria-label="设置" onClick={() => setShowSettings(true)}>
                        <Icon name="settings" size={16} />
                    </button>
                </div>

                <div className="search">
                    <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="搜索用户 / IP" />
                </div>

                <div className="list">
                    {filteredMembers.length === 0 && filteredHistory.length === 0 && (
                        <div className="empty">{running ? '未发现在线用户' : '点击「启动」开始'}</div>
                    )}
                    {groupedMembers.map((g) => (
                        <Fragment key={g.name}>
                            {showGroupHeaders && (
                                <div className="list-section">{g.name === DEFAULT_GROUP ? '在线用户' : g.name}</div>
                            )}
                            {g.members.map(contactRow)}
                        </Fragment>
                    ))}
                    {filteredHistory.length > 0 && <div className="list-section">历史会话</div>}
                    {filteredHistory.map((m) => {
                        const key = peerKey(m);
                        const n = unread[key] || 0;
                        return (
                            <div
                                key={key}
                                className={'contact offline' + (key === selectedKey ? ' active' : '')}
                                onClick={() => selectPeer(m, true)}
                            >
                                <div className="avatar-wrap">
                                    <Avatar name={m.name} size={38} />
                                    <span className="presence off" />
                                </div>
                                <div className="contact-info">
                                    <div className="contact-name">{m.name || m.ip}</div>
                                    <div className="contact-sub">
                                        <span className="mono">{m.ip}</span>
                                        {m.lastText && (
                                            <span className="preview" title={m.lastText}>
                                                {m.lastText}
                                            </span>
                                        )}
                                    </div>
                                </div>
                                {n > 0 && <span className="badge">{n > 99 ? '99+' : n}</span>}
                            </div>
                        );
                    })}
                </div>

                <div className="sidebar-actions">
                    <button onClick={refresh} disabled={!running}>
                        <Icon name="refresh" size={14} />
                        刷新在线用户
                    </button>
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
                                <div className={'chat-sub' + (typing[selectedKey] && !selectedOffline ? ' typing' : '')}>
                                    {selectedOffline ? (
                                        <>
                                            <span>离线</span>
                                            <span className="mono">{selected.ip}</span>
                                            <span className="tag">仅显示历史消息</span>
                                        </>
                                    ) : typing[selectedKey] ? (
                                        '正在输入…'
                                    ) : (
                                        <>
                                            {selected.group && selected.group !== DEFAULT_GROUP && (
                                                <span className="tag">{selected.group}</span>
                                            )}
                                            <span className="mono">{selected.ip}</span>
                                            {selected.absence && <span className="tag">离开</span>}
                                        </>
                                    )}
                                </div>
                            </div>
                            <div className="chat-actions">
                                <button
                                    className="ghost"
                                    title={selectedOffline ? '对方不在线' : '窗口抖动'}
                                    onClick={knock}
                                    disabled={selectedOffline}
                                >
                                    <Icon name="bell" size={14} />
                                    抖一抖
                                </button>
                                <button className="ghost" onClick={() => attach('file')} disabled={selectedOffline}>
                                    文件
                                </button>
                                <button className="ghost" onClick={() => attach('folder')} disabled={selectedOffline}>
                                    文件夹
                                </button>
                                <button
                                    className="ghost"
                                    onClick={sendShareLink}
                                    disabled={selectedOffline}
                                    title="发送我的共享文件夹链接"
                                >
                                    共享
                                </button>
                            </div>
                        </header>

                        <div className="messages">
                            {chat.length === 0 && !typing[selectedKey] && (
                                <div className="chat-empty">
                                    <Avatar name={selected.name} size={58} />
                                    <div className="chat-empty-title">和 {selected.name} 打个招呼</div>
                                    <div className="chat-empty-sub">
                                        <span className="mono">{selected.ip}</span>
                                        {selected.group && selected.group !== DEFAULT_GROUP && (
                                            <span className="tag">{selected.group}</span>
                                        )}
                                        <span>消息只在本机局域网传输</span>
                                    </div>
                                </div>
                            )}
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
                                                    {msg.text && <div className="text">{renderText(msg.text)}</div>}
                                                    {msg.files.map((f, j) => (
                                                        <div className="file" key={j}>📎 {f}</div>
                                                    ))}
                                                </div>
                                            )}
                                            <div className="stamp">{fmtStamp(msg.time)}</div>
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

                        {selectedOffline ? (
                            <footer className="composer composer-offline">
                                <span>对方当前不在线，这里仅显示已保存的历史消息。</span>
                            </footer>
                        ) : (
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
                        )}
                    </>
                ) : (
                    <div className="welcome">
                        <div className="welcome-logo"><PawMark size={56} /></div>
                        <div className="welcome-title">内网通</div>
                        <div className="welcome-sub">局域网通讯与文件传输</div>
                        <div className="welcome-hint">{running ? '从左侧选择一位用户开始聊天' : '点击右上角「启动」开始使用'}</div>
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
                        <span className="rail-arrow"><Icon name="chevronLeft" size={16} /></span>
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
                                        <span className="tab-ico"><Icon name={t.icon} size={15} /></span>
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
                                        <p className="hint">修改后需先停止再启动引擎才会生效。</p>

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

                                {settingsTab === 'share' && (
                                    <section className="pane">
                                        <h4>共享文件夹</h4>
                                        <p className="hint">
                                            同事收到链接后，用浏览器即可浏览和下载；开启“允许上传”后，同事也能把文件放进这个文件夹。
                                        </p>
                                        <label className="field">
                                            <span>共享目录</span>
                                            <div className="inline">
                                                <input
                                                    value={shareDir}
                                                    onChange={(e) => setShareDir(e.target.value)}
                                                    placeholder="选择要共享的文件夹"
                                                    disabled={!!shareInfo?.running || shareBusy}
                                                />
                                                <button className="btn" disabled={!!shareInfo?.running || shareBusy} onClick={pickShareDir}>
                                                    选择
                                                </button>
                                            </div>
                                        </label>
                                        <div className="inline">
                                            <label className="field" style={{flex: 1}}>
                                                <span>端口</span>
                                                <input
                                                    type="number"
                                                    min={1}
                                                    max={65535}
                                                    value={sharePort}
                                                    onChange={(e) => setSharePort(Number(e.target.value) || 0)}
                                                    disabled={!!shareInfo?.running || shareBusy}
                                                />
                                            </label>
                                            <label className="field row" style={{alignSelf: 'flex-end'}}>
                                                <input
                                                    type="checkbox"
                                                    checked={shareAllowUpload}
                                                    onChange={(e) => setShareAllowUpload(e.target.checked)}
                                                    disabled={!!shareInfo?.running || shareBusy}
                                                />
                                                <span>允许对方上传</span>
                                            </label>
                                        </div>
                                        <div className="inline">
                                            <button className="btn primary" disabled={shareBusy || !!shareInfo?.running} onClick={startShare}>
                                                {shareBusy ? '处理中…' : '开始共享'}
                                            </button>
                                            <button className="btn" disabled={shareBusy || !shareInfo?.running} onClick={stopShare}>
                                                停止共享
                                            </button>
                                        </div>
                                        {shareInfo?.running ? (
                                            <>
                                                <div className="divider">共享链接</div>
                                                {(shareInfo.urls || []).map((u) => (
                                                    <div className="share-url" key={u}>
                                                        <span className="share-url-text" title={u}>{u}</span>
                                                        <button className="link" onClick={() => copyText(u)}>复制</button>
                                                        <button className="link" onClick={() => OpenURL(u).catch(() => undefined)}>打开</button>
                                                    </div>
                                                ))}
                                                <p className="hint">
                                                    链接里带有随机访问令牌，拿到链接的人都能浏览{shareInfo.allowUpload ? '、下载和上传' : '和下载'}。停止共享后链接立即失效，重新开始会生成新链接。
                                                </p>
                                            </>
                                        ) : (
                                            <p className="hint">
                                                未开始共享。选择文件夹后点「开始共享」；首次使用需要在防火墙放行 TCP {sharePort || 2427}。
                                            </p>
                                        )}
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
                                                    placeholder="扫描网段，如 192.168.1. 或 192.168.1.0/24"
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
                                                <span>启用（到点自动断网；需点下方「应用」写入计划任务）</span>
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
                                                    ? `已生效：每天 ${nightOffStart} 禁用「${nightOffAdapterName}」，${nightOffEnd} 恢复（由 Windows 计划任务执行，内网通关闭也会生效）`
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
