import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { toast } from "react-toastify";

import AuthLayout from "../components/auth/AuthLayout";
import toastOptions from "../utils/toastOptions";
import { auth } from "../api/services";
import OidcButtons from "../components/auth/OidcButtons";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

export default function SignUp() {
  const { t, tError, locale } = useI18n();
  const { login, config } = useAuth();
  const navigate = useNavigate();

  const [form, setForm] = useState({
    name: "", surname: "", email: "", password: "", confirmPassword: "",
    // A claim, not a grant: choosing "coach" queues the account for a
    // superadmin to confirm, and it stays an athlete until they do.
    coach: false,
  });
  const [busy, setBusy] = useState(false);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async (event) => {
    event.preventDefault();
    if (form.password !== form.confirmPassword) {
      toast.error(t("errors.passwordsMismatch"), toastOptions);
      return;
    }
    setBusy(true);
    try {
      const response = await auth.register({
        email: form.email,
        password: form.password,
        name: form.name,
        surname: form.surname,
        coach: form.coach,
        locale,
      });
      // A brand-new account is disabled until it's activated (by email link or
      // by an administrator), except the very first one ever created - which
      // becomes the superadmin and can go straight in.
      if (response.role === "disabled") {
        toast.info(
          config?.activationMode === "email" ? t("auth.checkEmail") : t("errors.accountDisabledAdmin"),
          toastOptions
        );
        if (form.coach) toast.info(t("auth.coachRequestSent"), toastOptions);
        navigate("/login");
        return;
      }
      if (form.coach && response.coachRequest?.status === "pending") {
        toast.info(t("auth.coachRequestSent"), toastOptions);
      }
      await login(response);
      toast.success(t("auth.accountCreated"), toastOptions);
      navigate("/dashboard/feed");
    } catch (err) {
      toast.error(tError(err), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthLayout>
        <h1 style={{ textAlign: "center" }}>{t("auth.signup")}</h1>

        <form onSubmit={submit}>
          <div className="sf-row" style={{ gap: "0.6rem" }}>
            <div className="sf-field" style={{ flex: 1, minWidth: 140 }}>
              <label className="sf-label" htmlFor="name">
                {t("auth.name")}
              </label>
              <input id="name" className="sf-input" value={form.name} onChange={set("name")} required />
            </div>
            <div className="sf-field" style={{ flex: 1, minWidth: 140 }}>
              <label className="sf-label" htmlFor="surname">
                {t("auth.surname")}
              </label>
              <input id="surname" className="sf-input" value={form.surname} onChange={set("surname")} required />
            </div>
          </div>
          <div className="sf-field">
            <label className="sf-label" htmlFor="email">
              {t("auth.email")}
            </label>
            <input id="email" className="sf-input" type="email" value={form.email} onChange={set("email")} required />
          </div>

          {/* Which of the two this account is. Picking "coach" doesn't grant
              anything - it queues a request, because a coach writes other
              people's training. */}
          <div className="sf-field">
            <span className="sf-label">{t("auth.accountType")}</span>
            <div className="sf-choice-row">
              {[
                { value: false, label: t("auth.imAnAthlete"), help: t("auth.imAnAthleteHelp") },
                { value: true, label: t("auth.imACoach"), help: t("auth.imACoachHelp") },
              ].map((option) => (
                <label
                  key={String(option.value)}
                  className={`sf-choice ${form.coach === option.value ? "selected" : ""}`}
                >
                  <input
                    type="radio"
                    name="accountType"
                    checked={form.coach === option.value}
                    onChange={() => setForm((current) => ({ ...current, coach: option.value }))}
                  />
                  <span>
                    <strong>{option.label}</strong>
                    <span className="sf-muted">{option.help}</span>
                  </span>
                </label>
              ))}
            </div>
          </div>
          <div className="sf-field">
            <label className="sf-label" htmlFor="password">
              {t("auth.password")}
            </label>
            <input
              id="password"
              className="sf-input"
              type="password"
              autoComplete="new-password"
              value={form.password}
              onChange={set("password")}
              required
            />
          </div>
          <div className="sf-field">
            <label className="sf-label" htmlFor="confirmPassword">
              {t("auth.confirmPassword")}
            </label>
            <input
              id="confirmPassword"
              className="sf-input"
              type="password"
              autoComplete="new-password"
              value={form.confirmPassword}
              onChange={set("confirmPassword")}
              required
            />
          </div>
          <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy}>
            {busy ? t("common.loading") : t("auth.signup")}
          </button>
        </form>

        <OidcButtons />

        <div className="sf-auth-links">
          <span />
          <Link to="/login">{t("auth.hasAccount")}</Link>
        </div>
    </AuthLayout>
  );
}
