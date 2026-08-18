import { useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";

import toastOptions from "../utils/toastOptions";
import { contact } from "../api/services";
import AuthLayout from "../components/auth/AuthLayout";
import { ErrorMessage } from "../components/common/Feedback";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

const EMPTY = { firstname: "", name: "", email: "", subject: "", message: "" };

/**
 * The public contact form. It's deliberately reachable signed out - someone who
 * can't get into their account is exactly who most needs to reach us - so it
 * sits on the same split layout as the login screen.
 *
 * Submissions go to CWCloud's contact-request API through our own
 * `POST /v1/contact`, which is what holds the form id and the caller's IP; the
 * browser never talks to CWCloud directly.
 */
export default function Contact() {
  const { t } = useI18n();
  const { user } = useAuth();

  // Prefill the address for a signed-in user rather than making them retype one
  // the app already knows.
  const [form, setForm] = useState(() => ({ ...EMPTY, email: user?.email || "" }));
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await contact.send(form);
      toast.success(t("contact.sent"), toastOptions);
      setForm({ ...EMPTY, email: user?.email || "" });
      setSent(true);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthLayout>
      <h1 style={{ textAlign: "center" }}>{t("contact.title")}</h1>
      <p className="sf-muted" style={{ textAlign: "center" }}>
        {t("contact.body")}
      </p>

      {sent ? <div className="sf-notice">{t("contact.sent")}</div> : null}

      <form onSubmit={submit}>
        <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 140 }}>
            <label className="sf-label" htmlFor="firstname">
              {t("auth.name")} <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <input id="firstname" className="sf-input" value={form.firstname} onChange={set("firstname")} />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 140 }}>
            <label className="sf-label" htmlFor="name">
              {t("auth.surname")} <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <input id="name" className="sf-input" value={form.name} onChange={set("name")} />
          </div>
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="email">
            {t("auth.email")}
          </label>
          <input
            id="email"
            className="sf-input"
            type="email"
            autoComplete="email"
            value={form.email}
            onChange={set("email")}
            required
          />
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="subject">
            {t("contact.subject")}
          </label>
          <input id="subject" className="sf-input" value={form.subject} onChange={set("subject")} required />
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="message">
            {t("contact.message")}
          </label>
          <textarea
            id="message"
            className="sf-textarea"
            style={{ minHeight: 130 }}
            rows={5}
            value={form.message}
            onChange={set("message")}
            required
          />
        </div>

        <ErrorMessage error={error} />

        <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy}>
          {busy ? t("common.loading") : t("contact.send")}
        </button>
      </form>

      <div className="sf-auth-links">
        <Link to={user ? "/dashboard/feed" : "/login"}>
          {user ? t("nav.feed") : t("auth.login")}
        </Link>
        <Link to="/about">{t("nav.about")}</Link>
      </div>
    </AuthLayout>
  );
}
