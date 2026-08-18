import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import Avatar from "../components/common/Avatar";
import Logo from "../components/common/Logo";
import ShareButtons from "../components/common/ShareButtons";
import { ErrorMessage, Spinner } from "../components/common/Feedback";
import { publicPosts } from "../api/services";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";
import { postShareUrl, shareTextFor } from "../utils/shareText";

/**
 * One post, opened from a shared link.
 *
 * Readable with no account, which is the whole point: a link posted to
 * Instagram is followed by people who do not have one. Only posts published to
 * everybody resolve here - a club-only post answers 404, and says so rather
 * than asking the visitor to sign in for something they still would not be
 * allowed to read.
 */
export default function PublicPost() {
  const { postId } = useParams();
  const { t, locale } = useI18n();
  const { user } = useAuth();

  const [data, setData] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    publicPosts
      .get(postId)
      .then(setData)
      .catch(() => setError(t("feed.postUnavailable")));
  }, [postId, t]);

  const post = data?.post;

  return (
    <div className="sf-page" style={{ maxWidth: 680 }}>
      <div className="sf-page-header">
        <Link to={user ? "/dashboard/feed" : "/login"} aria-label={t("app.name")}>
          <Logo style={{ width: 150 }} />
        </Link>
        <Link className="sf-button sf-button-secondary" to={user ? "/dashboard/feed" : "/login"}>
          {user ? t("nav.feed") : t("auth.login")}
        </Link>
      </div>

      <ErrorMessage error={error} />

      {error ? null : !post ? (
        <Spinner />
      ) : (
        <>
          <div className="sf-card">
            <div className="sf-row" style={{ gap: "0.6rem", alignItems: "center" }}>
              <Avatar user={post.author} />
              <div>
                <Link to={`/profile/${post.author.handle}`}>
                  <strong>
                    {post.author.name} {post.author.surname}
                  </strong>
                </Link>
                <div className="sf-muted" style={{ fontSize: "0.82rem" }}>
                  {new Date(post.createdAt).toLocaleString(locale)}
                </div>
              </div>
            </div>

            {post.content ? (
              <p style={{ whiteSpace: "pre-wrap", marginTop: "0.8rem" }}>{post.content}</p>
            ) : null}

            {(post.pictures || []).map((picture, index) => (
              <img key={index} className="sf-post-picture" src={picture} alt="" />
            ))}

            {(post.links || []).map((link) => (
              <media-player key={link} url={link} />
            ))}

            <div className="sf-row" style={{ gap: "0.6rem", marginTop: "0.8rem" }}>
              <span className="sf-muted">{t("feed.likes", { count: post.likes || 0 })}</span>
              <span className="sf-muted">
                {t("feed.comments", { count: data.totalComments || 0 })}
              </span>
            </div>

            <ShareButtons url={postShareUrl(post.id)} text={shareTextFor(post.content, t("share.postText"))} />
          </div>

          {/* Read-only: replying needs an account, and pretending otherwise
              would put a box in front of somebody that cannot submit. */}
          {(data.comments || []).map((comment) => (
            <div key={comment.id} className="sf-comment">
              <Avatar user={comment.author} size="sf-avatar-sm" />
              <div className="sf-comment-body">
                <strong>
                  {comment.author.name} {comment.author.surname}
                </strong>
                <p style={{ margin: 0, whiteSpace: "pre-wrap" }}>{comment.content}</p>
              </div>
            </div>
          ))}

          {!user ? (
            <div className="sf-card" style={{ textAlign: "center" }}>
              <p className="sf-muted" style={{ marginTop: 0 }}>
                {t("feed.joinToReply")}
              </p>
              <Link className="sf-button" to="/signup">
                {t("auth.signup")}
              </Link>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
