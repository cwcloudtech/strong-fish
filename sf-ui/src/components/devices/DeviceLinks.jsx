import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { FiCpu, FiLink, FiRefreshCw, FiSlash, FiTrash2 } from "react-icons/fi";
import ReactMarkdown from "react-markdown";

import { ai as aiApi, devices as devicesApi } from "../../api/services";
import ExportMenu from "../programs/ExportMenu";
import downloadBlob from "../../utils/downloadBlob";
import longDate from "../../utils/longDate";
import { ConfirmModal } from "../common/Modal";
import AskButton from "../ai/AskButton";
import useToastError from "../../utils/useToast";
import { ErrorMessage, Spinner } from "../common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/** The windows the API offers, in the order they widen. */
const RANGES = ["24h", "48h", "week", "14d", "month", "90d"];

/** Ranges wide enough for a daily average to mean anything. */
const AVERAGED = new Set(["week", "14d", "month", "90d"]);

/**
 * How a day's count reads at a glance, on the usual walking scale.
 *
 * Applied to a day, never to a sum: ninety days of walking adds up to a number
 * that would sit in the top band whatever the member actually did, which is
 * the opposite of what colouring it is for. So the bars are coloured by their
 * own day, and the headline only when it is itself a daily figure - today's
 * count, or the average per 24h.
 */
function stepTone(steps) {
  if (steps < 6000) return "sf-steps-low";
  if (steps < 10000) return "sf-steps-fair";
  if (steps < 15000) return "sf-steps-good";
  return "sf-steps-high";
}

/** Every outcome the API's callback can redirect back with. */
const OUTCOMES = new Set([
  "connected",
  "denied",
  "not_linked",
  "invalid_state",
  "missing_code",
  "unavailable",
  "exchange_failed",
  "account_failed",
  "store_failed",
]);

// translate() falls back to the key itself for an unknown one, which would put
// "devices.outcome.something" on screen; a status this build does not know
// about reads as a plain failure instead.
function outcomeMessage(t, outcome) {
  return t(OUTCOMES.has(outcome) ? `devices.outcome.${outcome}` : "devices.outcome.exchange_failed");
}

/**
 * A member's connected health accounts, and what they counted.
 *
 * Steps are read rather than stored: StrongFish is not a step tracker, and a
 * copy of somebody's daily counts would be a second source of truth to keep in
 * sync with the watch that actually recorded them.
 *
 * The connection itself is made in a browser tab at the provider, so this
 * screen is also where the member comes back to - the API's callback redirects
 * here with the outcome in the query string, since the window that consented
 * belongs to Google, not to this app.
 */
export default function DeviceLinks() {
  const { t } = useI18n();
  const [connections, setConnections] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState("");
  const [params, setParams] = useSearchParams();

  const outcome = params.get("device") ? params.get("status") : "";

  const load = useCallback(async () => {
    try {
      setConnections(await devicesApi.list());
    } catch (err) {
      setError(err);
      setConnections([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // The outcome is read once and then taken out of the URL, or it would be
  // reported again on every later visit to this page.
  const dismiss = () => {
    params.delete("device");
    params.delete("status");
    setParams(params, { replace: true });
  };

  const connect = async (provider) => {
    setBusy(provider);
    setError(null);
    try {
      const { url } = await devicesApi.authorizeUrl(provider);
      // Same tab: the member is leaving to consent and the provider sends
      // them back here. A popup would be blocked as often as not.
      window.location.href = url;
    } catch (err) {
      setError(err);
      setBusy("");
    }
  };

  const disconnect = async (provider) => {
    setBusy(provider);
    setError(null);
    try {
      await devicesApi.disconnect(provider);
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setBusy("");
    }
  };

  if (connections === null) return <Spinner />;
  // Only when this build knows of no integration at all. A provider the
  // operator has not configured still appears, switched off: a blank space
  // leaves a member wondering whether the feature exists.
  if (connections.length === 0) return null;

  const activitySource = connections.find((connection) => connection.activitySource)?.provider;

  return (
    <div className="sf-card">
      <p className="sf-muted" style={{ marginTop: 0 }}>{t("devices.subtitle")}</p>

      {outcome ? (
        <div
          className={outcome === "connected" ? "sf-notice" : "sf-notice sf-notice-warning"}
          style={{ marginTop: "0.6rem" }}
          role="status"
        >
          {outcomeMessage(t, outcome)}{" "}
          <button className="sf-button sf-button-ghost sf-button-sm" onClick={dismiss}>
            {t("common.close")}
          </button>
        </div>
      ) : null}

      <ErrorMessage error={error} />

      {connections.map((connection) => (
        <div key={connection.provider} style={{ marginTop: "0.8rem" }}>
          <div className="sf-row-between">
            <div>
              <strong>{t(`devices.provider.${connection.provider}`)}</strong>
              <div className="sf-muted">
                {!connection.available
                  ? t("devices.unavailable")
                  : connection.connected
                    ? t("devices.connectedSince", { date: (connection.connectedAt || "").slice(0, 10) })
                    : t("devices.notConnected")}
              </div>
            </div>
            {connection.connected ? (
              <button
                className="sf-button sf-button-secondary sf-button-sm"
                onClick={() => disconnect(connection.provider)}
                disabled={busy === connection.provider}
              >
                <FiSlash /> {t("devices.disconnect")}
              </button>
            ) : (
              <button
                className="sf-button sf-button-sm"
                onClick={() => connect(connection.provider)}
                disabled={busy === connection.provider || !connection.available}
              >
                <FiLink /> {t("devices.connect")}
              </button>
            )}
          </div>

          {/* Steps and heart rate from one provider only, the one the API
              picked: two watches' worth of steps side by side would be two
              answers to the same question. */}
          {connection.activitySource ? (
            <>
              <DataPanel provider={connection.provider} />
              <HealthFeedbackPanel provider={connection.provider} />
            </>
          ) : null}
          {connection.connected && !connection.activitySource && activitySource
          && connection.capabilities?.includes("activity") ? (
            <p className="sf-muted" style={{ marginTop: "0.5rem" }}>
              {t("devices.activityFrom", { provider: t(`devices.provider.${activitySource}`) })}
            </p>
          ) : null}
          {connection.connected && connection.capabilities?.includes("body-composition") ? (
            <BodyCompositionPanel provider={connection.provider} />
          ) : null}
        </div>
      ))}
    </div>
  );
}

/**
 * What the AI made of the member's health data.
 *
 * Once a day, over ninety days: the window is wide because the question is a
 * direction rather than a day, and the wait is a day because that is how
 * often the numbers underneath move. Asking again an hour later would spend
 * tokens to be told the same thing.
 *
 * Kept until it is replaced. An old reading of a rolling window is out of
 * date rather than a second record, so there is one, and deleting it is how
 * you ask for another.
 */
function HealthFeedbackPanel({ provider }) {
  const { t, locale } = useI18n();
  const toastError = useToastError();
  const [feedback, setFeedback] = useState(null);
  const [busy, setBusy] = useState(false);
  // The subscriptions this member may spend, so the button can ask which one
  // pays when there is more than one.
  const [connections, setConnections] = useState([]);
  // Held until the member confirms: a reading cost money, and the next one
  // cannot be asked for until tomorrow.
  const [removing, setRemoving] = useState(false);

  useEffect(() => {
    // 404 is the ordinary case - nobody has asked yet - so it is not an error
    // to report, it is an empty panel with a button in it.
    devicesApi.healthFeedback(provider).then(setFeedback).catch(() => setFeedback(null));
    aiApi.connections().then(setConnections).catch(() => setConnections([]));
  }, [provider]);

  const generate = async (connectionId) => {
    setBusy(true);
    try {
      setFeedback(await devicesApi.generateHealthFeedback(provider, connectionId));
    } catch (err) {
      toastError(err);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    setBusy(true);
    try {
      await devicesApi.deleteHealthFeedback(provider);
      setFeedback(null);
    } catch (err) {
      toastError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="sf-ai-feedback" style={{ marginTop: "0.8rem" }}>
      <div className="sf-row-between">
        <strong>
          <FiCpu /> {t("devices.aiFeedback")}
        </strong>
        <div className="sf-row" style={{ gap: "0.3rem" }}>
          {/* Asked for as often as the member likes, through whichever
              subscription they pick: the reading replaces the one before it,
              since the ninety days it describes have moved on. */}
          <AskButton
            connections={connections}
            onAsk={generate}
            busy={busy}
            label={feedback ? t("devices.aiFeedbackAgain") : t("devices.aiFeedbackAsk")}
            tooltip={t("devices.aiFeedbackHelp")}
          />
          {feedback ? (
            <button
              type="button"
              className="sf-button-ghost sf-button-sm"
              onClick={() => setRemoving(true)}
              disabled={busy}
              aria-label={t("ai.deleteFeedback")}
              title={t("ai.deleteFeedback")}
            >
              <FiTrash2 />
            </button>
          ) : null}
        </div>
      </div>

      {feedback ? (
        <>
          <div className="sf-bubble-markdown">
            <ReactMarkdown>{feedback.content}</ReactMarkdown>
          </div>
          <p className="sf-ai-credit">
            {t("ai.generatedBy", {
              connection: feedback.connection || feedback.model || feedback.provider || "",
              date: longDate(locale, feedback.updatedAt),
            })}
            {feedback.days ? ` · ${t("devices.aiFeedbackWindow", { days: feedback.days })}` : ""}
          </p>
        </>
      ) : (
        <p className="sf-muted" style={{ margin: 0 }}>{t("devices.aiFeedbackEmpty")}</p>
      )}

      {removing ? (
        <ConfirmModal
          title={t("ai.deleteFeedback")}
          message={t("ai.deleteFeedbackConfirm")}
          confirmLabel={t("common.delete")}
          busy={busy}
          onClose={() => setRemoving(false)}
          onConfirm={() => {
            setRemoving(false);
            remove();
          }}
        />
      ) : null}
    </div>
  );
}

/**
 * One provider's step counts, in whichever window the member picks.
 *
 * Each range is a fresh read rather than one big fetch sliced locally: the
 * provider caps a request at ninety days, which is exactly the widest window
 * offered, so there is nothing to be gained by asking for more than what is
 * being looked at.
 */
function DataPanel({ provider }) {
  const { t, locale } = useI18n();
  const [range, setRange] = useState("week");
  const [exporting, setExporting] = useState(false);
  const [series, setSeries] = useState(null);
  const [heart, setHeart] = useState(null);
  const [heartError, setHeartError] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    // Apart, deliberately. The provider reaches back ninety days for steps
    // and only fourteen for heart rate, so one of the two is expected to be
    // missing on the wider views - and a single failed call must never blank
    // the steps the member came for.
    try {
      setSeries(await devicesApi.steps(provider, range));
    } catch (err) {
      setError(err);
      setSeries(null);
    }
    try {
      setHeart(await devicesApi.heartRate(provider, range));
      setHeartError(null);
    } catch (err) {
      setHeart(null);
      setHeartError(err);
    } finally {
      setLoading(false);
    }
  }, [provider, range]);

  useEffect(() => {
    load();
  }, [load]);

  const exportAs = async (format) => {
    setExporting(true);
    try {
      const blob = await devicesApi.export(provider, { format, range, locale });
      downloadBlob(blob, `health-${series?.to || range}.${format}`);
    } catch (err) {
      setError(err);
    } finally {
      setExporting(false);
    }
  };

  const peak = Math.max(1, ...(series?.days || []).map((day) => day.steps));

  // The most recent day the watch actually recorded - "last" rather than
  // "today", because today's reading only exists once it has been synced.
  const latest = [...(heart?.days || [])].reverse().find((day) => day.known) || null;

  return (
    <div style={{ marginTop: "0.8rem" }}>
      <div className="sf-row" style={{ gap: "0.3rem", flexWrap: "wrap" }}>
        {RANGES.map((name) => (
          <button
            key={name}
            className={`sf-button sf-button-sm ${range === name ? "" : "sf-button-secondary"}`}
            onClick={() => setRange(name)}
          >
            {t(`devices.range.${name}`)}
          </button>
        ))}
        <button
          className="sf-button sf-button-ghost sf-button-sm"
          onClick={load}
          disabled={loading}
          aria-label={t("common.refresh")}
          title={t("common.refresh")}
        >
          <FiRefreshCw />
        </button>
        {/* Exports what is on screen: the range picked above, not some other
            window the member would have to go and set again. */}
        <ExportMenu
          onExport={exportAs}
          busy={exporting}
          size="sf-button-sm"
          formats={["png", "pdf"]}
          label={t("devices.export")}
        />
      </div>

      <ErrorMessage error={error} />

      {loading && !series ? <Spinner /> : null}

      {series ? (
        <>
          <div className="sf-row" style={{ gap: "1.4rem", marginTop: "0.7rem", flexWrap: "wrap" }}>
            <div>
              <div className="sf-muted">{t("devices.total")}</div>
              {/* Coloured only over a single day, where the total is itself a
                  day's count. */}
              <strong
                className={`sf-steps-figure ${series.days.length === 1 ? stepTone(series.total) : ""}`.trim()}
              >
                {series.total.toLocaleString()}
              </strong>
            </div>
            {AVERAGED.has(series.range) ? (
              <div>
                <div className="sf-muted">{t("devices.dailyAverage")}</div>
                <strong className={`sf-steps-figure ${stepTone(series.dailyAverage)}`}>
                  {series.dailyAverage.toLocaleString()}
                </strong>
              </div>
            ) : null}
            {latest ? (
              <div>
                <div className="sf-muted">{t("devices.latestBpm")}</div>
                <strong className={`sf-steps-figure sf-zone-${latest.zone}`}>
                  {latest.avg}
                  <span className="sf-muted" style={{ fontSize: "0.8rem" }}> {t("devices.bpm")}</span>
                </strong>
              </div>
            ) : null}
          </div>

          {/* A bar per day, drawn with a div rather than a chart library: one
              series of plain numbers does not justify a dependency. */}
          <div className="sf-steps-chart" aria-hidden="true">
            {series.days.map((day) => (
              <div
                key={day.date}
                className={`sf-steps-bar ${stepTone(day.steps)}`}
                style={{ height: `${Math.round((day.steps / peak) * 100)}%` }}
                title={`${day.date}: ${day.steps.toLocaleString()}`}
              />
            ))}
          </div>

          {heart && !heart.clamped ? <HeartSection heart={heart} t={t} /> : null}
          {heart?.clamped ? (
            <p className="sf-muted" style={{ marginTop: "0.8rem" }}>
              {t("devices.heartRangeTooWide")}
            </p>
          ) : null}
          {!heart && heartError ? (
            <p className="sf-muted" style={{ marginTop: "0.8rem" }}>{t("devices.noHeartRate")}</p>
          ) : null}

          {/* The same numbers, readable: the bars are decoration, and a member
              using a screen reader is owed the counts themselves. */}
          <details style={{ marginTop: "0.5rem" }}>
            <summary className="sf-muted">{t("devices.perDay")}</summary>
            <div className="sf-table-wrapper">
              <table className="sf-table">
                <tbody>
                  {series.days.map((day) => (
                    <tr key={day.date}>
                      <td>{day.date}</td>
                      <td className={`sf-table-num sf-steps-figure ${stepTone(day.steps)}`}>
                        {day.steps.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </details>
        </>
      ) : null}
    </div>
  );
}

/**
 * Heart rate over the same days, each one coloured by the hardest zone it
 * reached.
 *
 * The zones are the provider's own four bands, not thresholds worked out here
 * from a birthday: a zone depends on somebody's maximum heart rate, and
 * guessing it would put a confident colour on a number that had not earned it.
 */
function HeartSection({ heart, t }) {
  const days = heart.days.filter((day) => day.known);
  if (days.length === 0) {
    return <p className="sf-muted" style={{ marginTop: "0.8rem" }}>{t("devices.noHeartRate")}</p>;
  }

  const peak = Math.max(1, ...days.map((day) => day.max || day.avg));

  return (
    <div style={{ marginTop: "1.1rem" }}>
      <div className="sf-row" style={{ gap: "1.4rem", flexWrap: "wrap" }}>
        <div>
          <div className="sf-muted">{t("devices.averageBpm")}</div>
          <strong className="sf-steps-figure">{heart.averageBpm}</strong>
        </div>
        <div>
          <div className="sf-muted">{t("devices.peakBpm")}</div>
          <strong className="sf-steps-figure">{heart.peakBpm}</strong>
        </div>
      </div>

      <div className="sf-steps-chart" aria-hidden="true">
        {heart.days.map((day) => (
          <div
            key={day.date}
            className={`sf-steps-bar sf-zone-${day.known ? day.zone : "none"}`}
            style={{ height: `${Math.round(((day.max || day.avg) / peak) * 100)}%` }}
            title={`${day.date}: ${day.avg} bpm (${t(`devices.zone.${day.zone}`)})`}
          />
        ))}
      </div>

      {/* What the colours mean, since four bands of colour explain nothing on
          their own. */}
      <div className="sf-row" style={{ gap: "0.9rem", flexWrap: "wrap", marginTop: "0.4rem" }}>
        {["light", "moderate", "vigorous", "peak"].map((zone) => (
          <span key={zone} className="sf-muted sf-zone-key">
            <i className={`sf-zone-dot sf-zone-${zone}`} /> {t(`devices.zone.${zone}`)}
          </span>
        ))}
      </div>
    </div>
  );
}

/**
 * The latest reading of every measure the member's scale takes.
 *
 * No chart and no range: this is "where am I now". Each measure carries its
 * own date, because a scale does not take every measure every time - fat is
 * only read barefoot - and a value from last month should say so.
 */
function BodyCompositionPanel({ provider }) {
  const { t } = useI18n();
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await devicesApi.bodyComposition(provider));
    } catch (err) {
      setError(err);
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [provider]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div style={{ marginTop: "1rem" }}>
      <div className="sf-row" style={{ gap: "0.3rem" }}>
        <strong>{t("devices.bodyComposition.title")}</strong>
        <button
          className="sf-button sf-button-ghost sf-button-sm"
          onClick={load}
          disabled={loading}
          aria-label={t("common.refresh")}
          title={t("common.refresh")}
        >
          <FiRefreshCw />
        </button>
      </div>

      <ErrorMessage error={error} />
      {loading && !data ? <Spinner /> : null}

      {data && data.measures.length === 0 ? (
        <p className="sf-muted">{t("devices.bodyComposition.empty")}</p>
      ) : null}

      {data && data.measures.length > 0 ? (
        <div className="sf-body-grid">
          {data.measures.map((measure) => (
            <div key={measure.type} className="sf-body-tile">
              <div className="sf-muted">{t(`devices.bodyComposition.measure.${measure.type}`)}</div>
              <strong className="sf-steps-figure">
                {measure.value.toLocaleString(undefined, { maximumFractionDigits: 1 })}
                {measure.unit ? (
                  <span className="sf-muted" style={{ fontSize: "0.8rem" }}>
                    {" "}
                    {measure.unit === "kg/m2" ? "kg/m²" : measure.unit}
                  </span>
                ) : null}
              </strong>
              <div className="sf-muted" style={{ fontSize: "0.75rem" }}>
                {t("devices.bodyComposition.takenOn", { date: (measure.takenAt || "").slice(0, 10) })}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
