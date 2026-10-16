import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { FiLink, FiRefreshCw, FiSlash } from "react-icons/fi";

import { devices as devicesApi } from "../../api/services";
import { ErrorMessage, Spinner } from "../common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/** The windows the API offers, in the order they widen. */
const RANGES = ["24h", "48h", "week", "month", "90d"];

/** Ranges wide enough for a daily average to mean anything. */
const AVERAGED = new Set(["week", "month", "90d"]);

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

          {connection.connected ? <DataPanel provider={connection.provider} /> : null}
        </div>
      ))}
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
  const { t } = useI18n();
  const [range, setRange] = useState("week");
  const [series, setSeries] = useState(null);
  const [heart, setHeart] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Fetched together: they are the same days of the same range, and two
      // panels that could disagree about which week they are showing would
      // be worse than one that is briefly empty.
      const [steps, rate] = await Promise.all([
        devicesApi.steps(provider, range),
        devicesApi.heartRate(provider, range),
      ]);
      setSeries(steps);
      setHeart(rate);
    } catch (err) {
      setError(err);
      setSeries(null);
      setHeart(null);
    } finally {
      setLoading(false);
    }
  }, [provider, range]);

  useEffect(() => {
    load();
  }, [load]);

  const peak = Math.max(1, ...(series?.days || []).map((day) => day.steps));

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

          {heart ? <HeartSection heart={heart} t={t} /> : null}

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
