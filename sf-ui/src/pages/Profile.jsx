import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { FiMessageSquare } from "react-icons/fi";

import { profiles } from "../api/services";
import Avatar from "../components/common/Avatar";
import PostCard from "../components/feed/PostCard";
import ShareButtons from "../components/common/ShareButtons";
import ProfileBadges from "../components/common/ProfileBadges";
import StrengthBadges from "../components/strength/StrengthBadges";
import StrengthTier from "../components/strength/StrengthTier";
import { filledSocials } from "../utils/socialProfiles";
import { EmptyState, ErrorMessage, Spinner } from "../components/common/Feedback";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

/**
 * A member's or coach's public profile. It works logged out - that's the point
 * of a shareable link - so nothing here assumes a session; the follow button and
 * the club-only posts simply don't appear for an anonymous visitor.
 */
export default function Profile() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const { handle } = useParams();

  const [profile, setProfile] = useState(null);
  const [posts, setPosts] = useState([]);
  const [error, setError] = useState(null);

  const load = useCallback(() => {
    profiles.get(handle).then(setProfile).catch(setError);
    profiles
      .posts(handle)
      .then((page) => setPosts(page?.results || []))
      .catch(() => setPosts([]));
  }, [handle]);

  useEffect(load, [load]);

  const toggleFollow = async () => {
    try {
      if (profile.followed) await profiles.unfollow(handle);
      else await profiles.follow(handle);
      setProfile(await profiles.get(handle));
    } catch (err) {
      setError(err);
    }
  };

  if (error) {
    return (
      <div className="sf-page">
        <EmptyState title={t("profile.notFound")} />
        <div style={{ textAlign: "center" }}>
          <Link className="sf-button" to={user ? "/dashboard/feed" : "/login"}>
            {user ? t("nav.feed") : t("auth.login")}
          </Link>
        </div>
      </div>
    );
  }
  if (!profile) return <Spinner />;

  const isSelf = user?.id === profile.id;
  const socials = filledSocials(profile.socials);

  return (
    <div className="sf-page" style={{ maxWidth: 720 }}>
      <div className="sf-card">
        <div className="sf-profile-header">
          <Avatar user={profile} size="sf-avatar-lg" />
          <div style={{ flex: 1, minWidth: 200 }}>
            <div className="sf-row-between">
              <div>
                <h1 style={{ marginBottom: 0 }}>
                  {profile.name} {profile.surname}
                </h1>
                <div className="sf-muted">
                  @{profile.handle}
                  {profile.bodyweight ? ` · ${profile.bodyweight} ${t("common.kg")}` : ""}
                </div>
                {/* Standing and specialty, each in its own colour: this is the
                    line somebody reads first, and "Coach" in grey text next to
                    the handle was not read at all. */}
                <ProfileBadges role={profile.role} />
              </div>
              {user && !isSelf ? (
                <div className="sf-row" style={{ gap: "0.4rem" }}>
                  {/* Reaching this page at all means the profile is visible to
                      the caller, which is exactly the API's condition for being
                      allowed to message its owner. */}
                  <Link className="sf-button sf-button-secondary" to={`/dashboard/messages?with=${profile.id}`}>
                    <FiMessageSquare /> {t("messages.message")}
                  </Link>
                  <button className={`sf-button ${profile.followed ? "sf-button-secondary" : ""}`} onClick={toggleFollow}>
                    {profile.followed ? t("profile.unfollow") : t("profile.follow")}
                  </button>
                </div>
              ) : null}
              {isSelf ? (
                <Link className="sf-button sf-button-secondary" to="/dashboard/settings">
                  {t("common.edit")}
                </Link>
              ) : null}
            </div>

            {profile.bio ? <p>{profile.bio}</p> : null}

            {/* Where else to find this lifter. Same table as the settings form,
                so a link here and the field that filled it cannot disagree.
                They open in a new tab: somebody leaving to look at an
                Instagram should come back to the profile they were reading. */}
            {socials.length ? (
              <div className="sf-profile-socials">
                {socials.map(({ key, label, Icon, account, href, rank }) => (
                  <a
                    key={key}
                    className="sf-social-link"
                    href={href}
                    target="_blank"
                    rel="noreferrer noopener"
                    title={rank ? `${label} · ${account} · ${t("profile.rank")} ${rank}` : `${label} · ${account}`}
                  >
                    <Icon aria-hidden="true" />
                    <span>{rank || label}</span>
                  </a>
                ))}
              </div>
            ) : null}

            {/* Only a profile somebody else can actually open is worth
                sharing: a link to a members-only profile would land a
                stranger on a 404. */}
            {profile.handle ? (
              <ShareButtons
                url={`${window.location.origin}/profile/${profile.handle}`}
                text={t("share.profileText", { name: `${profile.name} ${profile.surname}`.trim() })}
                label={t("share.label")}
              />
            ) : null}

            {/* What this lifter's own maxes are worth: their tier, where they
                sit among the club, and what they have earned. Absent for
                somebody who has not weighed in or entered no lifts - a profile
                with nothing to measure wears nothing. */}
            {profile.strength ? (
              <div className="sf-strength-profile">
                {/* The tier is the headline here, and where they sit among the
                    club is its tooltip: a percentile printed beside it would
                    compete with the thing it describes. */}
                <StrengthTier result={profile.strength} />
                <StrengthBadges result={profile.strength} earnedOnly />
              </div>
            ) : null}

            <div className="sf-profile-stats">
              <div>
                <span className="sf-stat-value">{profile.followers}</span>
                <span className="sf-stat-label">{t("profile.followers")}</span>
              </div>
              <div>
                <span className="sf-stat-value">{profile.following}</span>
                <span className="sf-stat-label">{t("profile.following")}</span>
              </div>
              {profile.clubs?.length ? (
                <div>
                  <span className="sf-stat-value">{profile.clubs.length}</span>
                  <span className="sf-stat-label">{t("profile.clubs")}</span>
                </div>
              ) : null}
            </div>
          </div>
        </div>

        {profile.bests?.length ? (
          <div className="sf-bests">
            {profile.bests.map((best) => (
              <div key={best.exerciseId} className="sf-best">
                <span className="sf-best-value">
                  {best.value} {t("common.kg")}
                </span>
                <span className="sf-stat-label">{best.labels?.[locale] || best.labels?.en || best.slug}</span>
              </div>
            ))}
            <div className="sf-best">
              <span className="sf-best-value">
                {profile.total} {t("common.kg")}
              </span>
              <span className="sf-stat-label">{t("profile.total")}</span>
            </div>
          </div>
        ) : null}

        {profile.clubs?.length ? (
          <p className="sf-muted" style={{ marginTop: "0.9rem", marginBottom: 0 }}>
            {t("profile.clubs")}: {profile.clubs.map((club) => `${club.name} (${t(`clubs.${club.role}`)})`).join(", ")}
          </p>
        ) : null}
      </div>

      <h2>{t("profile.posts")}</h2>
      {posts.length === 0 ? (
        <EmptyState message={t("profile.noPosts")} />
      ) : (
        posts.map((post) => (
          <PostCard
            key={post.id}
            post={post}
            onChanged={(updated) => setPosts((current) => current.map((item) => (item.id === updated.id ? updated : item)))}
            onDeleted={(postId) => setPosts((current) => current.filter((item) => item.id !== postId))}
          />
        ))
      )}

      <ErrorMessage error={error} />
    </div>
  );
}
