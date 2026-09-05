import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  ArrowClockwise24Regular,
  ArrowNext24Regular,
  ArrowPrevious24Regular,
  CalendarClock24Regular,
  CheckmarkCircle24Filled,
  Database24Regular,
  Delete24Regular,
  DismissCircle24Regular,
  FolderOpen24Regular,
  History24Regular,
  MusicNote224Regular,
  Pause24Filled,
  Play24Filled,
  PlugConnected24Regular,
  ShieldLock24Regular,
  SignOut24Regular,
  Speaker224Regular,
  Video24Regular,
} from "@fluentui/react-icons";

const WORKSPACE_VIEWS = new Set(["downloads", "history", "player", "account"]);
const HISTORY_KEY = "musicdl:playback-history";
const DOWNLOAD_PAGE_SIZE = 20;
const HISTORY_PAGE_SIZE = 12;

const normalizedView = (value) => WORKSPACE_VIEWS.has(value) ? value : "";

export function currentWorkspaceView() {
  return normalizedView(new URLSearchParams(window.location.search).get("view") || "");
}

export function openWorkspaceView(view, historyMode = "push") {
  const nextView = normalizedView(view);
  if (!nextView) return;

  ["downloadRecordsModal", "playbackHistoryModal", "cookieModal"].forEach((id) => {
    const modal = document.getElementById(id);
    if (modal) modal.style.display = "none";
    if (id === "cookieModal") document.body.classList.remove("settings-dialog-open");
  });

  const root = String(window.API_ROOT || "/music").replace(/\/$/, "");
  const url = new URL(window.location.href);
  url.pathname = `${root}/`;
  url.search = "";
  url.searchParams.set("view", nextView);
  if (historyMode === "replace") {
    window.history.replaceState({ musicDlWorkspace: nextView }, "", url);
  } else {
    window.history.pushState({ musicDlWorkspace: nextView }, "", url);
  }
  window.dispatchEvent(new CustomEvent("musicdl:workspace-change", { detail: nextView }));
  document.body.classList.remove("content-list-scroll-active");
  window.scrollTo({ top: 0, behavior: "auto" });
}

window.openMusicDlWorkspace = openWorkspaceView;

export function useWorkspaceView() {
  const [view, setView] = useState(currentWorkspaceView);

  useEffect(() => {
    const handleChange = (event) => setView(normalizedView(event.detail || ""));
    const handlePathChange = () => setView("");
    const handlePopState = () => setView(currentWorkspaceView());
    window.addEventListener("musicdl:workspace-change", handleChange);
    window.addEventListener("musicdl:path-change", handlePathChange);
    window.addEventListener("popstate", handlePopState);
    return () => {
      window.removeEventListener("musicdl:workspace-change", handleChange);
      window.removeEventListener("musicdl:path-change", handlePathChange);
      window.removeEventListener("popstate", handlePopState);
    };
  }, []);

  return view;
}

export const workspaceMetadata = {
  downloads: { group: "活动", title: "下载记录" },
  history: { group: "活动", title: "播放历史" },
  player: { group: "活动", title: "正在播放" },
  account: { group: "系统", title: "账户" },
};

function formatDate(value) {
  const date = new Date(value || 0);
  return Number.isNaN(date.getTime()) ? "-" : date.toLocaleString();
}

function formatTime(value) {
  const seconds = Math.max(0, Number(value || 0));
  if (!Number.isFinite(seconds)) return "0:00";
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${String(Math.floor(seconds % 60)).padStart(2, "0")}`;
}

function PageButton({ disabled, label, onClick, children }) {
  return <button type="button" className="workspace-icon-button" disabled={disabled} onClick={onClick} aria-label={label} title={label}>{children}</button>;
}

function EmptyState({ icon: Icon, title, detail, action }) {
  return (
    <div className="workspace-empty-state">
      <span><Icon aria-hidden="true" /></span>
      <strong>{title}</strong>
      {detail ? <p>{detail}</p> : null}
      {action || null}
    </div>
  );
}

function DownloadRecordsPage({ apiRoot }) {
  const [page, setPage] = useState(1);
  const [data, setData] = useState({ records: [], playlist_tasks: [], total: 0, total_pages: 1 });
  const [state, setState] = useState("loading");
  const [error, setError] = useState("");
  const recordsScrollRef = useRef(null);

  const load = useCallback(async (targetPage = page, silent = false) => {
    if (!silent) setState("loading");
    setError("");
    try {
      const params = new URLSearchParams({ page: String(targetPage), page_size: String(DOWNLOAD_PAGE_SIZE) });
      const response = await fetch(`${apiRoot}/api/downloads/records?${params}`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const next = await response.json();
      setData({ ...next, records: next.records || [], playlist_tasks: next.playlist_tasks || [] });
      setPage(Math.max(1, Number(next.page || targetPage)));
      setState("ready");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
      setState("error");
    }
  }, [apiRoot, page]);

  useEffect(() => { load(1); }, [apiRoot]);
  useEffect(() => {
    const refresh = () => load(1, true);
    window.addEventListener("musicdl:workspace-refresh", refresh);
    return () => window.removeEventListener("musicdl:workspace-refresh", refresh);
  }, [load]);
  const hasActivePlaylistTask = (data.playlist_tasks || []).some((task) => ["queued", "resolving", "downloading"].includes(task.status));
  useEffect(() => {
    if (!hasActivePlaylistTask) return undefined;
    const timer = window.setInterval(() => load(1, true), 2000);
    return () => window.clearInterval(timer);
  }, [hasActivePlaylistTask, load]);
  useEffect(() => {
    window.setDownloadRecordsButtonState?.(hasActivePlaylistTask ? "downloading" : "idle");
  }, [hasActivePlaylistTask]);

  useEffect(() => {
    recordsScrollRef.current?.scrollTo({ top: 0, behavior: "auto" });
  }, [page]);

  const clear = async () => {
    if (!window.confirm("确定清空所有下载记录？此操作不可撤销。")) return;
    const response = await fetch(`${apiRoot}/api/downloads/records`, { method: "DELETE" });
    if (!response.ok) {
      setError(`HTTP ${response.status}`);
      return;
    }
    await load(1);
  };

  const summary = useMemo(() => {
    const records = data.records || [];
    return {
      success: records.filter((record) => record.Status === "success").length,
      skipped: records.filter((record) => record.Status === "skipped").length,
      failed: records.filter((record) => record.Status === "failed").length,
    };
  }, [data.records]);

  const playlistTasks = data.playlist_tasks || [];
  const openLocalPlaylist = (collectionID) => {
    if (!collectionID) return;
    const url = `${apiRoot}/collection?id=${encodeURIComponent(collectionID)}`;
    if (typeof window.navigateTo === "function") window.navigateTo(url);
    else window.location.href = url;
  };

  return (
    <section className="workspace-view activity-workspace download-records-workspace" aria-label="下载记录">
      <div className="workspace-view-toolbar">
        <div className="activity-summary">
          <span><strong>{data.total || 0}</strong>全部</span>
          <span className="is-success"><strong>{summary.success}</strong>本页成功</span>
          <span className="is-warning"><strong>{summary.skipped}</strong>本页跳过</span>
          <span className="is-error"><strong>{summary.failed}</strong>本页失败</span>
        </div>
        <div className="workspace-toolbar-actions">
          <button type="button" className="workspace-command" onClick={() => window.openLocalMusicPage?.()}><FolderOpen24Regular />本地音乐</button>
          <PageButton label="刷新" onClick={() => load(page)}><ArrowClockwise24Regular /></PageButton>
          <PageButton label="清空下载记录" onClick={clear} disabled={!data.total}><Delete24Regular /></PageButton>
        </div>
      </div>

      {playlistTasks.length ? (
        <section className="playlist-download-tasks" aria-label="歌单下载任务">
          <header><div><strong>歌单下载</strong><span>完成后自动保存到“我的歌单”</span></div><span>{playlistTasks.length} 个任务</span></header>
          <div className="playlist-download-task-list">{playlistTasks.map((task) => {
            const total = Math.max(0, Number(task.total || 0));
            const completed = Math.max(0, Number(task.completed || 0));
            const percent = total > 0 ? Math.min(100, Math.round((completed / total) * 100)) : 0;
            const statusMeta = {
              queued: ["等待中", "is-queued"],
              resolving: ["解析歌单", "is-running"],
              downloading: ["下载中", "is-running"],
              completed: ["已完成", "is-success"],
              partial: ["部分完成", "is-warning"],
              failed: ["失败", "is-error"],
            }[task.status] || [task.status || "未知", "is-error"];
            return <article className="playlist-download-task" key={task.id}>
              <div className="playlist-task-main">
                <div className="playlist-task-title"><strong>{task.playlist_name || "未命名歌单"}</strong><span className="source-badge">{task.source || "-"}</span></div>
                <div className="playlist-task-progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent}><span style={{ width: `${percent}%` }} /></div>
                <div className="playlist-task-detail">
                  <span>{task.current_song || (task.status === "queued" ? "等待前一个任务完成" : total > 0 ? `${completed} / ${total}` : "正在读取歌曲列表")}</span>
                  {task.error ? <span className="is-error">{task.error}</span> : null}
                </div>
              </div>
              <div className="playlist-task-stats">
                <span className={`activity-status ${statusMeta[1]}`}>{statusMeta[0]}</span>
                <small>成功 {task.success || 0}</small><small>跳过 {task.skipped || 0}</small><small>失败 {task.failed || 0}</small>
                {task.local_collection_id ? <button type="button" className="workspace-command" onClick={() => openLocalPlaylist(task.local_collection_id)}><FolderOpen24Regular />打开歌单</button> : null}
              </div>
            </article>;
          })}</div>
        </section>
      ) : null}

      {state === "loading" ? <div className="workspace-loading">正在读取下载记录</div> : null}
      {state === "error" ? <div className="workspace-inline-error">加载失败：{error}</div> : null}
      {state === "ready" && !data.records.length && !playlistTasks.length ? <EmptyState icon={History24Regular} title="暂无下载记录" detail="下载任务完成后会显示在这里" /> : null}
      {data.records.length ? (
        <div className="activity-table-wrap activity-scroll-region" ref={recordsScrollRef} tabIndex="0" aria-label="下载歌曲记录列表">
          <table className="activity-table">
            <thead><tr><th>歌曲</th><th>歌手</th><th>来源</th><th>状态</th><th>时间</th></tr></thead>
            <tbody>{data.records.map((record, index) => {
              const status = record.Status === "success" ? "成功" : record.Status === "skipped" ? "跳过" : "失败";
              return <tr key={`${record.CreatedAt || "record"}-${index}`} title={record.Error || ""}>
                <td data-label="歌曲"><strong>{record.Name || "-"}</strong>{record.PlaylistName ? <small className="record-playlist-name">{record.PlaylistName}</small> : null}</td>
                <td data-label="歌手">{record.Artist || "-"}</td>
                <td data-label="来源"><span className="source-badge">{record.Source || "-"}</span></td>
                <td data-label="状态"><span className={`activity-status is-${record.Status || "failed"}`}>{status}</span></td>
                <td data-label="时间"><time>{formatDate(record.CreatedAt)}</time></td>
              </tr>;
            })}</tbody>
          </table>
        </div>
      ) : null}

      {Number(data.total_pages || 1) > 1 ? <div className="workspace-pagination">
        <PageButton label="上一页" disabled={page <= 1} onClick={() => load(page - 1)}><ArrowPrevious24Regular /></PageButton>
        <span>第 {page} / {data.total_pages} 页</span>
        <PageButton label="下一页" disabled={page >= data.total_pages} onClick={() => load(page + 1)}><ArrowNext24Regular /></PageButton>
      </div> : null}
    </section>
  );
}

function readHistory() {
  try {
    const entries = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
    return Array.isArray(entries) ? entries.filter((entry) => entry?.id && entry?.source && entry?.name) : [];
  } catch (_) {
    return [];
  }
}

function PlaybackHistoryPage() {
  const [entries, setEntries] = useState(readHistory);
  const [page, setPage] = useState(1);
  const historyScrollRef = useRef(null);
  const totalPages = Math.max(1, Math.ceil(entries.length / HISTORY_PAGE_SIZE));
  const visible = entries.slice((page - 1) * HISTORY_PAGE_SIZE, page * HISTORY_PAGE_SIZE);

  useEffect(() => {
    const refresh = () => setEntries(readHistory());
    window.addEventListener("storage", refresh);
    window.addEventListener("musicdl:playback-history-change", refresh);
    return () => {
      window.removeEventListener("storage", refresh);
      window.removeEventListener("musicdl:playback-history-change", refresh);
    };
  }, []);

  useEffect(() => {
    setPage((current) => Math.min(current, totalPages));
  }, [totalPages]);
  useEffect(() => {
    historyScrollRef.current?.scrollTo({ top: 0, behavior: "auto" });
  }, [page]);

  const clear = () => {
    if (!window.confirm("确定清空播放历史？")) return;
    localStorage.removeItem(HISTORY_KEY);
    setEntries([]);
    setPage(1);
  };

  const play = (entry) => {
    window.playPlaybackHistoryEntry?.(entry);
    openWorkspaceView("player");
  };

  return (
    <section className="workspace-view activity-workspace playback-history-workspace" aria-label="播放历史">
      <div className="workspace-view-toolbar">
        <div className="activity-summary"><span><strong>{entries.length}</strong>最近播放</span></div>
        <div className="workspace-toolbar-actions">
          <button type="button" className="workspace-command" onClick={() => openWorkspaceView("player")}><MusicNote224Regular />正在播放</button>
          <PageButton label="清空播放历史" onClick={clear} disabled={!entries.length}><Delete24Regular /></PageButton>
        </div>
      </div>

      {!entries.length ? <EmptyState icon={History24Regular} title="暂无播放历史" detail="播放过的歌曲会保存在此设备" /> : (
        <div className="history-list activity-scroll-region" ref={historyScrollRef} tabIndex="0" aria-label="播放历史歌曲列表">{visible.map((entry) => (
          <article className="history-row" key={`${entry.source}-${entry.id}`}>
            <div className="history-cover">{entry.cover ? <><img src={entry.cover} alt="" onError={(event) => { event.currentTarget.hidden = true; event.currentTarget.nextElementSibling?.removeAttribute("hidden"); }} /><MusicNote224Regular hidden /></> : <MusicNote224Regular />}</div>
            <div className="history-copy"><strong>{entry.name}</strong><span>{entry.artist || "未知歌手"}{entry.album ? ` · ${entry.album}` : ""}</span></div>
            <span className="source-badge">{entry.source}</span>
            <time>{formatDate(entry.playedAt)}</time>
            <button type="button" className="history-play-button" onClick={() => play(entry)} aria-label={`播放 ${entry.name}`} title="播放"><Play24Filled /></button>
          </article>
        ))}</div>
      )}

      {totalPages > 1 ? <div className="workspace-pagination">
        <PageButton label="上一页" disabled={page <= 1} onClick={() => setPage(page - 1)}><ArrowPrevious24Regular /></PageButton>
        <span>第 {page} / {totalPages} 页</span>
        <PageButton label="下一页" disabled={page >= totalPages} onClick={() => setPage(page + 1)}><ArrowNext24Regular /></PageButton>
      </div> : null}
    </section>
  );
}

function playerSnapshot() {
  const player = window.ap;
  const audios = player?.list?.audios || [];
  const index = Number(player?.list?.index || 0);
  const current = audios[index] || null;
  const audio = player?.audio;
  const lyricNodes = [...document.querySelectorAll(".aplayer-lrc p")];
  const hasAvailableLyrics = lyricNodes.some((node) => {
    const text = String(node.textContent || "").trim().toLowerCase();
    return text && text !== "not available";
  });
  document.body.classList.toggle("player-lyrics-unavailable", !hasAvailableLyrics);
  const activeLyric = lyricNodes.findIndex((node) => node.classList.contains("aplayer-lrc-current"));
  const audioDuration = Number(audio?.duration || 0);
  const declaredDuration = Number(current?.duration || 0);
  const duration = Math.max(
    Number.isFinite(audioDuration) ? audioDuration : 0,
    Number.isFinite(declaredDuration) ? declaredDuration : 0,
  );
  return {
    current,
    audios,
    index,
    playing: Boolean(audio && !audio.paused && current),
    currentTime: Number(audio?.currentTime || 0),
    duration,
    volume: Number(audio?.volume ?? 0.7),
    rate: Number(audio?.playbackRate || window.playerSpeed || 1),
    activeLyric,
    lyrics: lyricNodes
      .map((node, index) => ({
        index,
        text: node.textContent || "",
        active: node.classList.contains("aplayer-lrc-current"),
      }))
      .filter((line) => line.text.trim().toLowerCase() !== "not available"),
  };
}

export function MiniPlayer({ hidden = false }) {
  const [snapshot, setSnapshot] = useState(playerSnapshot);

  useEffect(() => {
    const update = () => setSnapshot(playerSnapshot());
    const interval = window.setInterval(update, 500);
    update();
    return () => window.clearInterval(interval);
  }, []);

  if (hidden || !snapshot.current) return null;
  const current = snapshot.current;
  const progress = snapshot.duration > 0
    ? Math.min(100, (snapshot.currentTime / snapshot.duration) * 100)
    : 0;

  return createPortal(
    <aside className="mini-player" aria-label="迷你播放器">
      <button type="button" className="mini-player-track" onClick={() => openWorkspaceView("player")} title="打开正在播放">
        <span className="mini-player-cover">{current.cover ? <><img src={current.cover} alt="" onError={(event) => { event.currentTarget.hidden = true; event.currentTarget.nextElementSibling?.removeAttribute("hidden"); }} /><MusicNote224Regular hidden /></> : <MusicNote224Regular />}</span>
        <span className="mini-player-copy"><strong>{current.name || "未知歌曲"}</strong><small>{current.artist || "未知歌手"}</small></span>
      </button>
      <button type="button" className="mini-player-control is-primary" onClick={() => window.ap?.toggle?.()} aria-label={snapshot.playing ? "暂停" : "播放"} title={snapshot.playing ? "暂停" : "播放"}>{snapshot.playing ? <Pause24Filled /> : <Play24Filled />}</button>
      <button type="button" className="mini-player-control" onClick={() => window.ap?.skipForward?.()} disabled={snapshot.audios.length < 2} aria-label="下一首" title="下一首"><ArrowNext24Regular /></button>
      <span className="mini-player-progress" aria-hidden="true"><span style={{ width: `${progress}%` }} /></span>
    </aside>,
    document.body,
  );
}

function AccountPage({ apiRoot }) {
  const [data, setData] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    const load = () => {
      setError("");
      fetch(`${apiRoot}/api/account`, {
        headers: { Accept: "application/json" },
        signal: controller.signal,
      })
        .then(async (response) => {
          const payload = await response.json().catch(() => null);
          if (response.status === 401) {
            const next = encodeURIComponent(`${apiRoot}/?view=account`);
            window.location.href = `${apiRoot}/login?next=${next}`;
            return null;
          }
          if (!response.ok) throw new Error(payload?.error || `HTTP ${response.status}`);
          setData(payload);
          return payload;
        })
        .catch((reason) => {
          if (reason?.name !== "AbortError") setError(reason instanceof Error ? reason.message : String(reason));
        });
    };
    load();
    window.addEventListener("musicdl:account-refresh", load);
    return () => {
      window.removeEventListener("musicdl:account-refresh", load);
      controller.abort();
    };
  }, [apiRoot]);

  if (error) {
    return <section className="workspace-view"><EmptyState icon={DismissCircle24Regular} title="账户信息加载失败" detail={error} action={<button type="button" className="workspace-command" onClick={() => window.location.reload()}>重试</button>} /></section>;
  }
  if (!data) {
    return <section className="workspace-view account-view is-loading" aria-label="账户"><div className="account-loading"><span /><span /><span /></div></section>;
  }

  const stats = [
    ["我的歌单", data.stats?.collections || 0, FolderOpen24Regular],
    ["本地音乐", data.stats?.local_tracks || 0, Database24Regular],
    ["下载记录", data.stats?.download_records || 0, ArrowClockwise24Regular],
    ["歌单任务", data.stats?.playlist_downloads || 0, MusicNote224Regular],
  ];
  const username = data.username || "本地用户";
  const initial = Array.from(username)[0]?.toUpperCase() || "U";
  const connectedPlatforms = (data.platforms || []).filter((platform) => platform.connected);
  const sessionIssued = data.session_issued_at ? formatDate(data.session_issued_at) : "本地免登录";
  const sessionExpires = data.session_expires_at ? formatDate(data.session_expires_at) : "不适用";

  return (
    <section className="workspace-view account-view" aria-label="账户概览">
      <header className="account-identity-band">
        <span className="account-avatar" aria-hidden="true">{initial}</span>
        <div className="account-identity-copy">
          <span>music-dl 账户</span>
          <h2>{username}</h2>
          <div><span className="account-role-badge">{data.role || "管理员"}</span><span className={data.auth_enabled ? "account-security-badge is-secure" : "account-security-badge"}><ShieldLock24Regular />{data.auth_enabled ? "密码登录已启用" : "本地免登录模式"}</span></div>
        </div>
        <button type="button" className="workspace-command account-platform-command" onClick={() => window.openPlatformAccountSettings?.()}><PlugConnected24Regular />管理平台账户</button>
      </header>

      <div className="account-stat-grid">{stats.map(([label, value, Icon]) => <article className="account-stat" key={label}><Icon aria-hidden="true" /><span>{label}</span><strong>{value}</strong></article>)}</div>

      <div className="account-detail-grid">
        <section className="account-section">
          <header><div><h3>平台连接</h3><span>{data.connected_platforms || 0} 个已连接</span></div><PlugConnected24Regular aria-hidden="true" /></header>
          {connectedPlatforms.length ? <div className="account-platform-list">{connectedPlatforms.map((platform) => <div className="account-platform-row" key={platform.source}><span className="account-platform-mark">{Array.from(platform.name || platform.source)[0]}</span><span><strong>{platform.name || platform.source}</strong><small>{platform.source}</small></span><CheckmarkCircle24Filled aria-label="已连接" /></div>)}</div> : <div className="account-section-empty">尚未连接音乐平台</div>}
        </section>

        <section className="account-section">
          <header><div><h3>当前会话</h3><span>{data.session_max_age_days || 7} 天有效期</span></div><ShieldLock24Regular aria-hidden="true" /></header>
          <dl className="account-session-list"><div><dt><CalendarClock24Regular />登录时间</dt><dd>{sessionIssued}</dd></div><div><dt><ShieldLock24Regular />到期时间</dt><dd>{sessionExpires}</dd></div></dl>
          <form action={`${apiRoot}/logout`} method="post" className="account-logout-form"><button type="submit" className="workspace-command is-danger"><SignOut24Regular />退出登录</button></form>
        </section>
      </div>
    </section>
  );
}

function NowPlayingPage() {
  const [snapshot, setSnapshot] = useState(playerSnapshot);
  const lyricsScrollRef = useRef(null);

  useEffect(() => {
    const update = () => setSnapshot(playerSnapshot());
    const interval = window.setInterval(update, 400);
    const audio = window.ap?.audio;
    ["play", "pause", "loadedmetadata", "durationchange", "volumechange"].forEach((event) => audio?.addEventListener(event, update));
    update();
    return () => {
      window.clearInterval(interval);
      ["play", "pause", "loadedmetadata", "durationchange", "volumechange"].forEach((event) => audio?.removeEventListener(event, update));
    };
  }, []);

  useEffect(() => {
    const container = lyricsScrollRef.current;
    const activeLine = container?.querySelector('[data-active="true"]');
    if (!container || !activeLine) return;
    const top = activeLine.offsetTop
      - (container.clientHeight - activeLine.offsetHeight) / 2;
    container.scrollTo({
      top: Math.max(0, top),
      behavior: "smooth",
    });
  }, [snapshot.activeLyric, snapshot.current?.custom_id, snapshot.current?.id]);

  const player = window.ap;
  const current = snapshot.current;
  const switchTrack = (index) => {
    player?.list?.switch?.(index);
    player?.play?.();
    window.setTimeout(() => setSnapshot(playerSnapshot()), 50);
  };
  const openVideo = () => {
    if (!current || !window.VideoGen) return;
    window.VideoGen.open({
      id: current.custom_id || current.id,
      source: current.source || "netease",
      name: current.name,
      artist: current.artist,
      album: current.album || "",
      cover: current.cover || "",
      duration: Number(current.duration || 0),
      extra: current.extra || "",
    });
  };

  if (!current) {
    return <section className="workspace-view now-playing-empty"><EmptyState icon={MusicNote224Regular} title="尚未播放音乐" detail="从搜索结果、歌单或本地音乐中选择一首歌曲" action={<button type="button" className="workspace-primary-command" onClick={() => window.navigateTo?.(`${window.API_ROOT}/`)}>前往发现</button>} /></section>;
  }

  const artworkStyle = current.cover
    ? { backgroundImage: `url(${JSON.stringify(current.cover)})` }
    : undefined;

  return (
    <section className="workspace-view now-playing-view" aria-label="正在播放">
      <div className="now-playing-ambient" style={artworkStyle} aria-hidden="true" />
      <div className="now-playing-layout">
        <div className="now-playing-main">
          <div className="now-playing-feature">
            <div className="now-playing-artwork-column">
              <div className="now-playing-cover">{current.cover ? <><img src={current.cover} alt="" onError={(event) => { event.currentTarget.hidden = true; event.currentTarget.nextElementSibling?.removeAttribute("hidden"); }} /><MusicNote224Regular hidden /></> : <MusicNote224Regular />}</div>
              <div className="now-playing-meta">
                <span className="now-playing-label">正在播放</span>
                <h2>{current.name || "未知歌曲"}</h2>
                <p className="now-playing-artist">{current.artist || "未知歌手"}</p>
                <p className="now-playing-album">{current.album || "未知专辑"} <span>·</span> {current.source || "音乐"}</p>
              </div>
            </div>

            <section className="lyrics-panel player-main-lyrics" aria-label="实时歌词">
              <header><h2>实时歌词</h2><span>{snapshot.lyrics.length ? `${snapshot.lyrics.length} 行` : "LYRICS"}</span></header>
              <div className="player-lyrics-scroll" ref={lyricsScrollRef}>
                {snapshot.lyrics.length
                  ? snapshot.lyrics.map((line) => <p className={line.active ? "is-active" : ""} data-active={line.active ? "true" : undefined} key={`${line.index}-${line.text}`}>{line.text || " "}</p>)
                  : <p className="lyrics-unavailable">暂无歌词</p>}
              </div>
            </section>
          </div>

          <div className="player-dock">
            <div className="player-timeline">
              <input type="range" min="0" max={Math.max(1, snapshot.duration)} step="0.1" value={Math.min(snapshot.currentTime, Math.max(1, snapshot.duration))} onChange={(event) => player?.seek?.(Number(event.target.value))} aria-label="播放进度" />
              <div><span>{formatTime(snapshot.currentTime)}</span><span>{formatTime(snapshot.duration)}</span></div>
            </div>

            <div className="player-dock-actions">
              <div className="player-dock-spacer" aria-hidden="true" />
              <div className="player-controls">
                <PageButton label="上一首" onClick={() => player?.skipBack?.()} disabled={snapshot.audios.length < 2}><ArrowPrevious24Regular /></PageButton>
                <button type="button" className="player-primary-button" onClick={() => player?.toggle?.()} aria-label={snapshot.playing ? "暂停" : "播放"} title={snapshot.playing ? "暂停" : "播放"}>{snapshot.playing ? <Pause24Filled /> : <Play24Filled />}</button>
                <PageButton label="下一首" onClick={() => player?.skipForward?.()} disabled={snapshot.audios.length < 2}><ArrowNext24Regular /></PageButton>
              </div>

              <div className="player-options">
                <label><Speaker224Regular aria-hidden="true" /><input type="range" min="0" max="1" step="0.01" value={snapshot.volume} onChange={(event) => player?.volume?.(Number(event.target.value), true)} aria-label="音量" /></label>
                <label><span>速度</span><select value={snapshot.rate} onChange={(event) => window.setPlayerPlaybackRate?.(Number(event.target.value))}>{(window.PLAYER_SPEEDS || [0.75, 1, 1.25, 1.5, 2]).map((rate) => <option value={rate} key={rate}>{rate}x</option>)}</select></label>
                <button type="button" className="player-video-button" onClick={openVideo} aria-label="生成视频" title="生成视频"><Video24Regular /></button>
              </div>
            </div>
          </div>
        </div>

        <aside className="now-playing-side">
          <section className="queue-panel player-side-content" aria-label="播放列表">
            <header><div><h2>播放列表</h2><p>接下来播放</p></div><span>{snapshot.audios.length} 首</span></header>
            <div>{snapshot.audios.map((audio, index) => <button type="button" className={index === snapshot.index ? "is-active" : ""} onClick={() => switchTrack(index)} key={`${audio.source || "audio"}-${audio.custom_id || audio.id || index}`}>
              <span>{index === snapshot.index && snapshot.playing ? <Pause24Filled /> : <Play24Filled />}</span>
              <span><strong>{audio.name || "未知歌曲"}</strong><small>{audio.artist || "未知歌手"}</small></span>
              <time>{formatTime(audio.duration)}</time>
            </button>)}</div>
          </section>
        </aside>
      </div>
    </section>
  );
}

export function WorkspaceView({ view, apiRoot }) {
  if (view === "downloads") return <DownloadRecordsPage apiRoot={apiRoot} />;
  if (view === "history") return <PlaybackHistoryPage />;
  if (view === "player") return <NowPlayingPage />;
  if (view === "account") return <AccountPage apiRoot={apiRoot} />;
  return null;
}
