import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { FiSearch, FiUsers } from "react-icons/fi";

import Avatar from "../../components/common/Avatar";
import { search as searchApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

const EMPTY = { terms: "", name: "", surname: "", email: "" };

/**
 * Finding people, shaped like uprodit's search: a free-text box plus the
 * criteria you can narrow by when you know something specific, combined with
 * AND.
 *
 * Results are whatever the API decided the caller may see - the visibility
 * rules run inside its query, so nothing is filtered here and the counts are
 * honest. A profile someone hid is not merely absent from the page, it never
 * counted towards the total.
 */
export default function Search() {
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();
  const [form, setForm] = useState(() => ({
    terms: params.get("terms") || "",
    name: params.get("name") || "",
    surname: params.get("surname") || "",
    email: params.get("email") || "",
  }));
  const [advanced, setAdvanced] = useState(
    Boolean(params.get("name") || params.get("surname") || params.get("email"))
  );
  const [results, setResults] = useState(null);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const criteria = Object.fromEntries(
    ["terms", "name", "surname", "email"].map((key) => [key, params.get(key) || ""])
  );
  const hasCriteria = Object.values(criteria).some(Boolean);

  const run = useCallback(async () => {
    if (!Object.values(criteria).some(Boolean)) {
      setResults(null);
      setTotal(0);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const page = await searchApi.members(criteria);
      setResults(page.results || []);
      setTotal(page.totalResults || 0);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
    // criteria is rebuilt each render from the URL, so depend on the URL itself.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [params]);

  useEffect(() => {
    run();
  }, [run]);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  // The query lives in the URL so a search can be shared, reloaded and gone
  // back to - and so the browser's own history works the way it looks like it
  // should.
  const submit = (event) => {
    event.preventDefault();
    const next = {};
    Object.entries(form).forEach(([key, value]) => {
      if (value.trim()) next[key] = value.trim();
    });
    setParams(next);
  };

  const reset = () => {
    setForm(EMPTY);
    setParams({});
  };

  return (
    <div className="sf-page" style={{ maxWidth: 820 }}>
      <h1 className="sf-title">{t("search.title")}</h1>
      <p className="sf-subtitle">{t("search.subtitle")}</p>

      <form className="sf-card" onSubmit={submit}>
        <div className="sf-field">
          <label className="sf-label" htmlFor="terms">
            {t("search.terms")}
          </label>
          <input
            id="terms"
            className="sf-input"
            value={form.terms}
            onChange={set("terms")}
            placeholder={t("search.termsPlaceholder")}
            autoFocus
          />
        </div>

        {advanced ? (
          <div className="sf-row" style={{ gap: "0.6rem" }}>
            <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
              <label className="sf-label" htmlFor="name">
                {t("auth.name")}
              </label>
              <input id="name" className="sf-input" value={form.name} onChange={set("name")} />
            </div>
            <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
              <label className="sf-label" htmlFor="surname">
                {t("auth.surname")}
              </label>
              <input id="surname" className="sf-input" value={form.surname} onChange={set("surname")} />
            </div>
            <div className="sf-field" style={{ flex: 1, minWidth: 180 }}>
              <label className="sf-label" htmlFor="email">
                {t("auth.email")}
              </label>
              <input id="email" className="sf-input" type="email" value={form.email} onChange={set("email")} />
            </div>
          </div>
        ) : null}

        <div className="sf-row" style={{ gap: "0.4rem" }}>
          <button className="sf-button" type="submit" disabled={busy}>
            <FiSearch /> {t("search.submit")}
          </button>
          <button
            type="button"
            className="sf-button sf-button-secondary"
            onClick={() => setAdvanced((open) => !open)}
          >
            {advanced ? t("search.simple") : t("search.advanced")}
          </button>
          {hasCriteria ? (
            <button type="button" className="sf-button-ghost" onClick={reset}>
              {t("search.clear")}
            </button>
          ) : null}
        </div>
      </form>

      <ErrorMessage error={error} />

      {busy ? (
        <Spinner />
      ) : results === null ? (
        <EmptyState title={t("search.startTitle")} message={t("search.startBody")} />
      ) : results.length === 0 ? (
        <EmptyState title={t("search.noneTitle")} message={t("search.noneBody")} />
      ) : (
        <>
          <p className="sf-muted">{t("search.results", { count: total })}</p>
          <ul className="sf-list">
            {results.map((member) => (
              <li className="sf-list-item" key={member.id}>
                <Avatar user={member} size="sf-avatar-sm" />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <strong>
                    {member.name} {member.surname}
                  </strong>
                  <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                    {member.handle ? `@${member.handle}` : null}
                    {member.sharesClub ? (
                      <span style={{ marginLeft: member.handle ? "0.5rem" : 0 }}>
                        <FiUsers style={{ verticalAlign: "-2px" }} /> {t("search.sharesClub")}
                      </span>
                    ) : null}
                  </div>
                </div>
                {member.handle ? (
                  <Link className="sf-button sf-button-secondary sf-button-sm" to={`/profile/${member.handle}`}>
                    {t("search.openProfile")}
                  </Link>
                ) : null}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
