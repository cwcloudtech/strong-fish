import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import Logo from "../components/common/Logo";
import LanguageDropdown from "../components/common/LanguageDropdown";
import { EmptyState, ErrorMessage, Spinner } from "../components/common/Feedback";
import { exerciseLabel } from "../components/training/SessionDay";
import { publicPrograms } from "../api/services";
import { describeLoad, formatReps } from "../utils/setFormat";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

/**
 * A program its coach published, read by anybody holding the link.
 *
 * It shows the prescription and nothing else: reps, effort and the coach's
 * notes, with no weights. Loads here are always derived from the reader's own
 * maxes (see the API's loadcalc), and an anonymous reader has none - so rather
 * than invent numbers, the page says what was prescribed and invites them to
 * sign in and run it against their own.
 */
export default function PublicProgram() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const { programId } = useParams();
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;
    publicPrograms
      .get(programId)
      .then((result) => !cancelled && setData(result))
      .catch((err) => !cancelled && setError(err));
    return () => {
      cancelled = true;
    };
  }, [programId]);

  const program = data?.program;

  return (
    <div className="sf-page" style={{ maxWidth: 900 }}>
      <div className="sf-page-header">
        <Link to={user ? "/dashboard/feed" : "/login"} aria-label={t("app.name")}>
          <Logo style={{ width: 150 }} />
        </Link>
        <div className="sf-row" style={{ gap: "0.6rem", alignItems: "center" }}>
          <LanguageDropdown />
          <Link className="sf-button sf-button-secondary" to={user ? "/dashboard/feed" : "/login"}>
            {user ? t("nav.feed") : t("auth.login")}
          </Link>
        </div>
      </div>

      {error ? (
        <div className="sf-card">
          <EmptyState title={t("publicProgram.notFoundTitle")} message={t("publicProgram.notFoundBody")} />
        </div>
      ) : data === null ? (
        <Spinner />
      ) : (
        <>
          <div className="sf-card">
            <h1 className="sf-title" style={{ marginBottom: "0.2rem" }}>
              {program.name}
            </h1>
            <p className="sf-muted" style={{ marginTop: 0 }}>
              {[program.clubName, program.authorName?.trim()].filter(Boolean).join(" · ")}
            </p>
            {program.description ? <p>{program.description}</p> : null}
            <p className="sf-muted" style={{ marginBottom: 0 }}>
              {t("programs.weeks", { count: program.weeks })} ·{" "}
              {t("programs.sessions", { count: program.dayCount })} ·{" "}
              {t("programs.setCount", { count: program.setCount })}
            </p>
          </div>

          <div className="sf-notice">{t("publicProgram.loadsNotice")}</div>

          <ErrorMessage error={error} />

          {(data.days || []).map((day) => (
            <div className="sf-card" key={day.id}>
              <h3 style={{ marginTop: 0 }}>{day.title}</h3>
              <div className="sf-table-wrapper">
                <table className="sf-table">
                  <thead>
                    <tr>
                      <th>{t("session.exercise")}</th>
                      <th className="sf-table-num">{t("session.reps")}</th>
                      <th>{t("session.loadMode")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(day.sets || []).map((set) => (
                      <tr key={set.id}>
                        <td>
                          {exerciseLabel(set, locale)}
                          {set.notes ? <div className="sf-muted">{set.notes}</div> : null}
                        </td>
                        <td className="sf-table-num">{formatReps(set)}</td>
                        <td className="sf-muted">{describeLoad(t, set)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ))}
        </>
      )}
    </div>
  );
}
