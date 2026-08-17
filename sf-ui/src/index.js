import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";

import App from "./App";
import "./index.css";
import { defineMediaPlayer } from "./webcomponents/MediaPlayer";

// Registered once, before the first render: post cards use <media-player> to
// turn a pasted video URL into an actual player.
defineMediaPlayer();

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>
);
