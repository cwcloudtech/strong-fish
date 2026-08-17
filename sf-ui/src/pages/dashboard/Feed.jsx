import { useCallback, useEffect, useRef, useState } from "react";

import { clubs as clubsApi, social } from "../../api/services";
import PostCard from "../../components/feed/PostCard";
import PostComposer from "../../components/feed/PostComposer";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The newspaper: posts from the people you follow, your own, and your clubs'.
 * "Discover" switches to every public post, which is what a brand-new account
 * needs before it follows anyone.
 *
 * Pages load on scroll rather than with a "more" button, matching how a feed is
 * expected to behave.
 */
export default function Feed() {
  const { t } = useI18n();
  const [tab, setTab] = useState("following");
  const [posts, setPosts] = useState([]);
  const [page, setPage] = useState(0);
  const [total, setTotal] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadedOnce, setLoadedOnce] = useState(false);
  const [error, setError] = useState(null);
  const [clubs, setClubs] = useState([]);
  const sentinel = useRef(null);

  const hasMore = total === null || posts.length < total;

  useEffect(() => {
    clubsApi.list().then(setClubs).catch(() => setClubs([]));
  }, []);

  // Switching tabs restarts paging from scratch.
  useEffect(() => {
    setPosts([]);
    setPage(0);
    setTotal(null);
    setLoadedOnce(false);
    setLoading(true);
  }, [tab]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const result = tab === "discover" ? await social.discover(page) : await social.feed(page);
        if (cancelled) return;
        setPosts((current) => (page === 0 ? result.results : [...current, ...result.results]));
        setTotal(result.totalResults);
      } catch (err) {
        if (!cancelled) setError(err);
      } finally {
        if (!cancelled) {
          setLoading(false);
          setLoadedOnce(true);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [page, tab]);

  useEffect(() => {
    const node = sentinel.current;
    if (!node || !loadedOnce || loading || !hasMore) return;
    const observer = new IntersectionObserver(
      (entries) => entries[0].isIntersecting && setPage((current) => current + 1),
      { rootMargin: "200px" }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [loadedOnce, loading, hasMore]);

  const replacePost = useCallback((updated) => {
    setPosts((current) => current.map((post) => (post.id === updated.id ? updated : post)));
  }, []);

  const removePost = useCallback((postId) => {
    setPosts((current) => current.filter((post) => post.id !== postId));
    setTotal((current) => (current === null ? current : Math.max(0, current - 1)));
  }, []);

  const prependPost = useCallback((post) => {
    setPosts((current) => [post, ...current]);
    setTotal((current) => (current === null ? current : current + 1));
  }, []);

  return (
    <div className="sf-page">
      <div className="sf-feed">
        <div className="sf-page-header">
          <h1>{t("feed.title")}</h1>
        </div>

        <div className="sf-tabs">
          <button type="button" className={`sf-tab ${tab === "following" ? "active" : ""}`} onClick={() => setTab("following")}>
            {t("feed.following")}
          </button>
          <button type="button" className={`sf-tab ${tab === "discover" ? "active" : ""}`} onClick={() => setTab("discover")}>
            {t("feed.discover")}
          </button>
        </div>

        <PostComposer clubs={clubs} onPosted={prependPost} />

        <ErrorMessage error={error} />

        {posts.map((post) => (
          <PostCard key={post.id} post={post} onChanged={replacePost} onDeleted={removePost} />
        ))}

        {loadedOnce && posts.length === 0 && !loading ? (
          <EmptyState message={tab === "discover" ? t("feed.emptyDiscover") : t("feed.empty")} />
        ) : null}

        {loading ? <Spinner /> : null}
        {loadedOnce && hasMore ? <div ref={sentinel} style={{ height: 1 }} /> : null}
      </div>
    </div>
  );
}
