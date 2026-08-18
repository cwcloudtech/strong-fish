import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { FiArrowLeft } from "react-icons/fi";
import Markdown from "react-markdown";

import Logo from "../components/common/Logo";
import { ErrorMessage, Spinner } from "../components/common/Feedback";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

/**
 * The About page, rendered from a Markdown file served alongside the build
 * (`public/about.<locale>.md`, falling back to `public/about.md`).
 *
 * It's fetched at runtime rather than imported, so the text can be edited - or
 * translated - by dropping a new file next to the built assets, with no
 * rebuild. That's also why the page has to cope with the file being missing or
 * unreadable rather than assuming it parsed.
 */
export default function About() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [content, setContent] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;

    const fetchFirst = async () => {
      // The localized file wins; the base one is the fallback, so a translation
      // is optional.
      for (const path of [`/about.${locale}.md`, "/about.md"]) {
        try {
          const response = await fetch(path, { headers: { Accept: "text/markdown, text/plain" } });
          if (!response.ok) continue;
          const text = await response.text();
          // A dev server (and some static hosts) answer an unknown path with
          // the SPA's index.html rather than a 404, which would otherwise be
          // rendered as a wall of escaped markup.
          if (text.trimStart().startsWith("<!DOCTYPE") || text.includes("<div id=\"root\">")) continue;
          return text;
        } catch {
          // Try the next candidate; only an exhausted list is an error.
        }
      }
      throw new Error("not-found");
    };

    fetchFirst()
      .then((text) => !cancelled && setContent(text))
      .catch(() => !cancelled && setError(t("about.unavailable")));

    return () => {
      cancelled = true;
    };
  }, [locale, t]);

  return (
    <div className="sf-page" style={{ maxWidth: 760 }}>
      <div className="sf-page-header">
        <Link to={user ? "/dashboard/feed" : "/login"} aria-label={t("app.name")}>
          <Logo style={{ width: 170 }} />
        </Link>
        {/* Back to wherever the reader came from - the page is reachable from
            the sidebar, from the signed-out footer and from a shared link, so
            a fixed destination would be wrong for two of the three. History
            can be empty (the link was opened in a new tab), and then the home
            the reader does have is the fallback. */}
        <div className="sf-row" style={{ gap: "0.5rem" }}>
          <button
            type="button"
            className="sf-button sf-button-secondary"
            onClick={() =>
              window.history.length > 1 ? navigate(-1) : navigate(user ? "/dashboard/feed" : "/login")
            }
          >
            <FiArrowLeft /> {t("common.back")}
          </button>
          <Link className="sf-button sf-button-secondary" to={user ? "/dashboard/feed" : "/login"}>
            {user ? t("nav.feed") : t("auth.login")}
          </Link>
        </div>
      </div>

      <div className="sf-card">
        {error ? (
          <ErrorMessage error={error} />
        ) : content === null ? (
          <Spinner />
        ) : (
          <div className="sf-markdown">
            <Markdown
              components={{
                // Anything the page links out to opens in its own tab, and
                // carries the usual protection against reverse tabnabbing.
                a: ({ href, children, ...props }) => {
                  const external = /^https?:\/\//i.test(href || "");
                  return (
                    <a
                      href={href}
                      {...props}
                      {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
                    >
                      {children}
                    </a>
                  );
                },
              }}
            >
              {content}
            </Markdown>
          </div>
        )}
      </div>
    </div>
  );
}
