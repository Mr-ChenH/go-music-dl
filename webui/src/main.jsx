import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { FluentProvider, webLightTheme } from "@fluentui/react-components";
import {
  ChevronUp16Regular,
  Clock24Regular,
  Dismiss20Regular,
  FolderAdd24Regular,
  FolderOpen24Regular,
  Grid24Regular,
  Heart24Regular,
  History24Regular,
  Home24Regular,
  MoreHorizontal24Regular,
  MusicNote224Regular,
  Person24Regular,
  Search24Regular,
  Settings24Regular,
  Sparkle24Regular,
} from "@fluentui/react-icons";
import "./styles.css";
import "./legacy.css";
import "./workspace.css";
import "./player-theme.css";
import "./player-layout-v2.css";
import {
  openWorkspaceView,
  MiniPlayer,
  useWorkspaceView,
  WorkspaceView,
  workspaceMetadata,
} from "./workspace.jsx";

const parseJSON = (value, fallback) => {
  try {
    return JSON.parse(value || "");
  } catch (_) {
    return fallback;
  }
};

const invoke = (name, ...args) => {
  const fn = window[name];
  if (typeof fn === "function") fn(...args);
};

const searchTypes = [
  { value: "song", label: "单曲", icon: MusicNote224Regular },
  { value: "playlist", label: "歌单", icon: Grid24Regular },
  { value: "album", label: "专辑", icon: FolderOpen24Regular },
];

function SearchConsole({ node }) {
  const data = node.dataset;
  const sources = parseJSON(data.sources, []);
  const selected = new Set(parseJSON(data.selected, []));
  const defaults = new Set(parseJSON(data.defaults, []));
  const descriptions = parseJSON(data.descriptions, {});
  const playlistSupported = parseJSON(data.playlistSupported, {});
  const albumSupported = parseJSON(data.albumSupported, {});
  const categorySupported = parseJSON(data.categorySupported, {});
  const userPlaylistSupported = parseJSON(data.userPlaylistSupported, {});
  const initialType = data.searchType || "song";
  const [searchType, setSearchType] = useState(initialType);
  const workspaceView = useWorkspaceView();
  const path = data.currentPath || window.location.pathname;
  const showSearchTools = !workspaceView && (path === data.root || path === `${data.root}/` || path.includes("/search"));
  const routeWorkspace = path.includes("/local_music_page")
    ? { group: "资料库", title: "本地音乐" }
    : path.includes("/my_collections")
      ? { group: "资料库", title: "我的歌单" }
      : path.includes("/collections") || path.includes("/collection")
      ? { group: "资料库", title: data.collectionName || "我的歌单" }
      : path.includes("/playlist_categories") || path.includes("/category_playlists")
        ? { group: "发现", title: "歌单分类" }
        : path.includes("/user_playlists")
          ? { group: "发现", title: "平台歌单" }
          : path.includes("/recommend")
            ? { group: "发现", title: "每日推荐" }
            : path.includes("/album")
              ? { group: "发现", title: "专辑详情" }
              : path.includes("/playlist")
                ? { group: "发现", title: "歌单详情" }
                : { group: "发现", title: "搜索与发现" };
  const workspace = workspaceView ? workspaceMetadata[workspaceView] : routeWorkspace;
  node.classList.toggle("is-dedicated-workspace", !showSearchTools);
  document.body.classList.toggle("react-workspace-page-active", Boolean(workspaceView));
  document.body.dataset.workspaceView = workspaceView || "";

  const supportsType = (source) => {
    if (searchType === "playlist") return Boolean(playlistSupported[source]);
    if (searchType === "album") return Boolean(albumSupported[source]);
    return true;
  };

  const handleTypeChange = (type) => {
    setSearchType(type);
    window.setTimeout(() => invoke("toggleSearchType", type), 0);
  };

  return (
    <div className="search-console">
      <header className="workspace-page-header">
        <div>
          <span className="workspace-kicker">{workspace.group}</span>
          <h1>{workspace.title}</h1>
        </div>
        <span className="workspace-status"><span aria-hidden="true" /> 服务可用</span>
      </header>

      {workspaceView ? (
        <WorkspaceView view={workspaceView} apiRoot={data.root} />
      ) : showSearchTools ? (
      <form action={`${data.root}/search`} method="get" id="search-form">
        <input type="hidden" name="page" value="1" />
        <input type="hidden" name="page_size" id="page-size-hidden" value={data.pageSize} />

        <section className="search-module fluent-panel" aria-labelledby="search-module-title">
          <div className="module-heading search-module-heading">
            <div>
              <span className="module-icon"><Search24Regular aria-hidden="true" /></span>
              <h2 id="search-module-title">聚合搜索</h2>
            </div>
            <div className="search-type-switch" role="radiogroup" aria-label="搜索类型">
              {searchTypes.map(({ value, label, icon: Icon }) => (
                <label className={`type-option${searchType === value ? " is-active" : ""}`} key={value}>
                  <input
                    type="radio"
                    name="type"
                    value={value}
                    checked={searchType === value}
                    onChange={() => handleTypeChange(value)}
                  />
                  <Icon aria-hidden="true" />
                  <span>{label}</span>
                </label>
              ))}
            </div>
          </div>

          <div className="input-group">
            <Search24Regular className="search-field-icon" aria-hidden="true" />
            <input
              type="text"
              id="search-keyword"
              name="q"
              defaultValue={data.keyword}
              placeholder={data.placeholder}
              required
              autoComplete="off"
            />
            <button type="submit" className="search-btn">
              <Search24Regular aria-hidden="true" />
              <span>搜索</span>
            </button>
          </div>
        </section>

        {data.error ? <div className="error-msg"><span aria-hidden="true">!</span>{data.error}</div> : null}

        <section className="source-selector fluent-panel" id="source-selector" aria-labelledby="source-module-title">
          <div className="source-header">
            <button type="button" className="source-collapse-btn" onClick={() => invoke("toggleSourceSelector")} aria-expanded="true" aria-controls="source-grid">
              <span className="module-icon"><Grid24Regular aria-hidden="true" /></span>
              <span className="source-heading-copy">
                <span className="source-title" id="source-module-title">搜索源</span>
                <span className="source-count">{sources.length} 个服务</span>
              </span>
              <ChevronUp16Regular className="source-collapse-icon" aria-hidden="true" />
            </button>
            <div className="source-actions">
              <button type="button" className="ctrl-btn" id="btn-all">全选</button>
              <button type="button" className="ctrl-btn" id="btn-none">清空</button>
            </div>
          </div>

          <div className="source-grid" id="source-grid">
            {sources.map((source) => {
              const isChecked = selected.size ? selected.has(source) : defaults.has(source);
              return (
                <label className="source-option" key={source}>
                  <input
                    type="checkbox"
                    name="sources"
                    value={source}
                    className="source-checkbox"
                    data-playlist-supported={String(Boolean(playlistSupported[source]))}
                    data-album-supported={String(Boolean(albumSupported[source]))}
                    data-category-supported={String(Boolean(categorySupported[source]))}
                    data-user-playlist-supported={String(Boolean(userPlaylistSupported[source]))}
                    defaultChecked={isChecked}
                    disabled={!supportsType(source)}
                  />
                  <span className="source-card">
                    <span className="source-check" aria-hidden="true">✓</span>
                    <span className="source-name">{source}</span>
                    <span className="source-desc">{descriptions[source]}</span>
                  </span>
                </label>
              );
            })}
          </div>
        </section>
      </form>
      ) : data.error ? (
        <div className="error-msg workspace-error"><span aria-hidden="true">!</span>{data.error}</div>
      ) : null}
    </div>
  );
}

function NavButton({ label, icon: Icon, onClick, active = false, className = "", id, children }) {
  return (
    <button
      id={id}
      type="button"
      className={`app-nav-button${active ? " is-active" : ""}${className ? ` ${className}` : ""}`}
      onClick={onClick}
      title={label}
      aria-label={label}
    >
      <Icon aria-hidden="true" />
      <span>{label}</span>
      {children}
    </button>
  );
}

function AppNavigation({ node }) {
  const data = node.dataset;
  const [path, setPath] = useState(data.currentPath || window.location.pathname);
  const workspaceView = useWorkspaceView();
  const [moreOpen, setMoreOpen] = useState(false);
  useEffect(() => {
    const handlePathChange = (event) => {
      setPath(String(event.detail || window.location.pathname));
    };
    window.addEventListener("musicdl:path-change", handlePathChange);
    return () => window.removeEventListener("musicdl:path-change", handlePathChange);
  }, []);
  const go = (name, ...args) => () => {
    setMoreOpen(false);
    invoke(name, ...args);
  };
  const goHome = () => {
    setMoreOpen(false);
    invoke("navigateTo", `${data.root}/`);
  };
  const goWorkspace = (view) => () => {
    setMoreOpen(false);
    openWorkspaceView(view);
  };
  const isPath = (...parts) => parts.some((part) => path.includes(part));

  const accountButton = <NavButton label="账户" icon={Person24Regular} onClick={goWorkspace("account")} active={workspaceView === "account"} />;

  return (
    <>
      <div className="sidebar-brand">
        <span className="sidebar-brand-mark"><img src={`${data.root}/icon.png`} alt="" /></span>
        <span><strong>music-dl</strong><small>音乐工作台</small></span>
      </div>

      <nav className="desktop-navigation" aria-label="应用模块">
        <section className="nav-group">
          <h2>发现</h2>
          <NavButton label="搜索与发现" icon={Search24Regular} onClick={goHome} active={!workspaceView && (path === `${data.root}/` || path === data.root || isPath("/search"))} />
          <NavButton label="每日推荐" icon={Sparkle24Regular} onClick={go("goToRecommend")} active={!workspaceView && isPath("/recommend")} />
          <NavButton label="歌单分类" icon={Grid24Regular} onClick={go("goToPlaylistCategories")} active={!workspaceView && isPath("/playlist_categories", "/category_playlists")} />
          <NavButton label="平台歌单" icon={FolderOpen24Regular} onClick={go("goToUserPlaylists")} active={!workspaceView && isPath("/user_playlists")} />
        </section>

        <section className="nav-group">
          <h2>资料库</h2>
          <NavButton label="我的歌单" icon={Heart24Regular} onClick={go("openCollectionManager")} active={!workspaceView && isPath("/my_collections", "/collections", "/collection")} />
          <NavButton label="本地音乐" icon={MusicNote224Regular} onClick={go("openLocalMusicPage")} active={!workspaceView && isPath("/local_music_page")} />
        </section>

        <section className="nav-group">
          <h2>活动</h2>
          <NavButton label="正在播放" icon={MusicNote224Regular} onClick={goWorkspace("player")} active={workspaceView === "player"} />
          <NavButton id="download-records-button" label="下载记录" icon={History24Regular} className="rt-btn-download-records" onClick={goWorkspace("downloads")} active={workspaceView === "downloads"}>
            <span className="rt-download-records-dot" aria-hidden="true" />
          </NavButton>
          <NavButton label="播放历史" icon={Clock24Regular} onClick={goWorkspace("history")} active={workspaceView === "history"} />
        </section>
      </nav>

      <div className="sidebar-spacer" />

      <div className="desktop-navigation sidebar-system-actions">
        <NavButton label="系统设置" icon={Settings24Regular} onClick={go("openSystemConfig")} />
        {accountButton}
      </div>

      <MiniPlayer hidden={workspaceView === "player"} />

      <nav className="mobile-navigation" aria-label="移动端导航">
        <NavButton label="首页" icon={Home24Regular} onClick={goHome} active={!workspaceView && (path === `${data.root}/` || path === data.root || isPath("/search"))} />
        <NavButton label="正在播放" icon={MusicNote224Regular} onClick={goWorkspace("player")} active={workspaceView === "player"} />
        <NavButton label="本地音乐" icon={FolderOpen24Regular} onClick={go("openLocalMusicPage")} active={!workspaceView && isPath("/local_music_page")} />
        <NavButton label="下载" icon={History24Regular} onClick={goWorkspace("downloads")} active={workspaceView === "downloads"} />
        <NavButton label="更多" icon={MoreHorizontal24Regular} onClick={() => setMoreOpen((value) => !value)} active={moreOpen || workspaceView === "history" || workspaceView === "account" || (!workspaceView && isPath("/recommend", "/playlist_categories", "/category_playlists", "/user_playlists", "/my_collections", "/collections", "/collection"))} />
      </nav>

      {moreOpen ? (
        <div className="mobile-more-panel" role="dialog" aria-label="更多功能">
          <header><strong>更多功能</strong><button type="button" onClick={() => setMoreOpen(false)} aria-label="关闭"><Dismiss20Regular /></button></header>
          <div className="mobile-more-grid">
            <NavButton label="每日推荐" icon={Sparkle24Regular} onClick={go("goToRecommend")} />
            <NavButton label="歌单分类" icon={Grid24Regular} onClick={go("goToPlaylistCategories")} />
            <NavButton label="我的歌单" icon={Heart24Regular} onClick={go("openCollectionManager")} />
            <NavButton label="平台歌单" icon={FolderOpen24Regular} onClick={go("goToUserPlaylists")} />
            <NavButton label="播放历史" icon={Clock24Regular} onClick={goWorkspace("history")} />
            <NavButton label="系统设置" icon={Settings24Regular} onClick={go("openSystemConfig")} />
            <NavButton label="账户" icon={Person24Regular} onClick={goWorkspace("account")} />
          </div>
        </div>
      ) : null}
    </>
  );
}

const mountedRoots = new WeakMap();

function mountNode(node, Component) {
  if (!node || mountedRoots.has(node)) return;
  const root = createRoot(node);
  mountedRoots.set(node, root);
  flushSync(() => {
    root.render(<FluentProvider theme={webLightTheme}><Component node={node} /></FluentProvider>);
  });
}

window.mountMusicDlReact = function mountMusicDlReact(scope = document) {
  const searchRoot = scope.querySelector?.("#react-search-root");
  mountNode(searchRoot, SearchConsole);
  const toolbar = document.querySelector("#react-navigation-root");
  mountNode(toolbar, AppNavigation);
};

window.mountMusicDlReact(document);
