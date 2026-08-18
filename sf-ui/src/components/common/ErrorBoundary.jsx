import { Component } from "react";

/**
 * Stops one screen's crash from blanking the whole app.
 *
 * Without a boundary, React unmounts the entire tree when a render throws - the
 * page, the sidebar, everything - and the user is left on a white screen with
 * no navigation and nothing to click. That failure mode is worse than the bug
 * that caused it: a broken page you can navigate away from is recoverable, a
 * white void is not.
 *
 * It deliberately does not try to be clever about recovery. The state that
 * caused the throw is still there, so re-rendering the same page would throw
 * again; what it offers is a way out and a way to retry deliberately.
 */
export default class ErrorBoundary extends Component {
  state = { error: null };

  static getDerivedStateFromError(error) {
    return { error };
  }

  componentDidCatch(error, info) {
    // Kept to the console rather than sent anywhere: this app has no error
    // reporting service, and inventing one here would be a decision the
    // deployment should make, not this component.
    console.error("Unhandled error while rendering", error, info?.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="sf-page" style={{ maxWidth: 620 }}>
        <div className="sf-card">
          <h2 style={{ marginTop: 0 }}>{this.props.title}</h2>
          <p className="sf-muted">{this.props.message}</p>
          <div className="sf-row" style={{ gap: "0.5rem" }}>
            <button className="sf-button" onClick={() => window.location.reload()}>
              {this.props.retryLabel}
            </button>
          </div>
        </div>
      </div>
    );
  }
}
