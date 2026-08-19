import { useEffect, useRef, useState } from "react";
import { FiMic, FiSquare, FiTrash2 } from "react-icons/fi";

import Tooltip from "../common/Tooltip";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Records a voice message and hands the blob back.
 *
 * MediaRecorder chooses its own container: Chrome and Firefox produce webm,
 * Safari mp4. Rather than insisting on one - which would mean transcoding in
 * the browser - the recorder reports what it actually produced and the upload
 * accepts either, since both play back everywhere that matters.
 *
 * Nothing is uploaded until the message is sent: a recording somebody discards
 * should never have reached anyone's storage.
 */

/** Ordered by preference; the first the browser admits to is used. */
const PREFERRED_TYPES = ["audio/webm", "audio/mp4", "audio/ogg"];

function supportedType() {
  if (typeof MediaRecorder === "undefined") return null;
  return PREFERRED_TYPES.find((type) => MediaRecorder.isTypeSupported?.(type)) ?? "";
}

export default function VoiceRecorder({ recording, onRecordingChange, disabled }) {
  const { t } = useI18n();
  const recorderRef = useRef(null);
  const chunksRef = useRef([]);
  const [active, setActive] = useState(false);
  const [seconds, setSeconds] = useState(0);
  const [error, setError] = useState(null);

  // Releasing the microphone matters: a live track leaves the browser's
  // recording indicator on, which reads as the app still listening.
  useEffect(() => {
    return () => {
      recorderRef.current?.stream?.getTracks().forEach((track) => track.stop());
    };
  }, []);

  useEffect(() => {
    if (!active) return undefined;
    const timer = setInterval(() => setSeconds((current) => current + 1), 1000);
    return () => clearInterval(timer);
  }, [active]);

  const start = async () => {
    setError(null);
    const mimeType = supportedType();
    if (mimeType === null) {
      setError(t("messages.recordingUnsupported"));
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
      chunksRef.current = [];

      recorder.ondataavailable = (event) => event.data.size && chunksRef.current.push(event.data);
      recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        const blob = new Blob(chunksRef.current, { type: recorder.mimeType || "audio/webm" });
        // A recording with nothing in it is a mis-tap, not a message.
        onRecordingChange(blob.size > 0 ? { blob, url: URL.createObjectURL(blob) } : null);
      };

      recorder.start();
      recorderRef.current = recorder;
      setSeconds(0);
      setActive(true);
    } catch {
      // Refusing the microphone is a decision, not a failure - say what it
      // means and leave the rest of the composer working.
      setError(t("messages.microphoneDenied"));
    }
  };

  const stop = () => {
    recorderRef.current?.stop();
    setActive(false);
  };

  const discard = () => {
    if (recording?.url) URL.revokeObjectURL(recording.url);
    onRecordingChange(null);
  };

  if (recording) {
    return (
      <div className="sf-row" style={{ gap: "0.4rem", alignItems: "center" }}>
        <audio className="sf-voice-preview" src={recording.url} controls />
        <Tooltip label={t("common.delete")}>
          <button type="button" className="sf-button-ghost" onClick={discard} aria-label={t("common.delete")}>
            <FiTrash2 />
          </button>
        </Tooltip>
      </div>
    );
  }

  return (
    <>
      <Tooltip label={active ? t("messages.stopRecording") : t("messages.record")}>
        <button
          type="button"
          className="sf-button-ghost"
          onClick={active ? stop : start}
          disabled={disabled}
          aria-label={active ? t("messages.stopRecording") : t("messages.record")}
          style={active ? { color: "var(--sf-danger)" } : undefined}
        >
          {active ? <FiSquare /> : <FiMic />}
        </button>
      </Tooltip>
      {active ? <span className="sf-muted sf-voice-timer">{formatSeconds(seconds)}</span> : null}
      {error ? <span className="sf-error sf-voice-error">{error}</span> : null}
    </>
  );
}

function formatSeconds(total) {
  const minutes = Math.floor(total / 60);
  return `${minutes}:${String(total % 60).padStart(2, "0")}`;
}
