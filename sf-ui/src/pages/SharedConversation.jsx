import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";

import ExportMenu from "../components/programs/ExportMenu";
import downloadBlob from "../utils/downloadBlob";
import { ai as aiApi } from "../api/services";
import { ErrorMessage, Spinner } from "../components/common/Feedback";
import { useI18n } from "../i18n/I18nContext";

/**
 * A conversation somebody shared, read from its link.
 *
 * No session needed, which is the point: a link that only works for members
 * is not a shared link. What can be read is decided by the API - the token is
 * the whole credential, and its owner can revoke it - so this page only has
 * to render what comes back.
 */
export default function SharedConversation() {
  const { t } = useI18n();
  const { token } = useParams();
  const [conversation, setConversation] = useState(null);
  const [error, setError] = useState(null);
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    aiApi.shared(token).then(setConversation).catch(setError);
  }, [token]);

  const exportAs = async (format) => {
    setExporting(true);
    try {
      const blob = await aiApi.exportShared(token, format);
      downloadBlob(blob, `${(conversation?.title || "conversation").replace(/[^\w-]+/g, "-")}.${format}`);
    } catch (err) {
      setError(err);
    } finally {
      setExporting(false);
    }
  };

  if (error) return <div className="sf-page"><ErrorMessage error={error} /></div>;
  if (!conversation) return <Spinner />;

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{conversation.title}</h1>
          <p className="sf-subtitle">{t("ai.sharedSubtitle")}</p>
        </div>
        <ExportMenu
          onExport={exportAs}
          busy={exporting}
          label={t("ai.exportConversation")}
          formats={["pdf", "png"]}
        />
      </div>

      {(conversation.exchanges || []).map((exchange) => (
        <div className="sf-card" key={exchange.id}>
          <strong>{exchange.question}</strong>
          <div className="sf-bubble-markdown" style={{ marginTop: "0.6rem" }}>
            <ReactMarkdown>{exchange.answer}</ReactMarkdown>
          </div>
        </div>
      ))}
    </div>
  );
}
