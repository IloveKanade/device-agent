import { useEffect, useState } from 'react';
import './App.css';
import { EventsOn } from "../wailsjs/runtime/runtime";
import { GetStatus, OpenURL } from "../wailsjs/go/main/App";

interface Status {
  connected: boolean;
  last_err?: string;
}

function App() {
  const [url, setURL] = useState("about:blank");
  const [status, setStatus] = useState<Status>({ connected: false });

  useEffect(() => {
    // Listen for URL change events
    const unsubscribeURL = EventsOn("open-url", (newUrl: string) => {
      console.log("Opening URL:", newUrl);
      setURL(newUrl);
    });

    // Listen for status change events
    const unsubscribeStatus = EventsOn("status", (newStatus: Status) => {
      console.log("Status changed:", newStatus);
      setStatus(newStatus);
    });

    // Get initial status
    GetStatus().then(setStatus).catch(console.error);

    return () => {
      unsubscribeURL();
      unsubscribeStatus();
    };
  }, []);

  const handleOpenURL = () => {
    const newUrl = prompt("Enter URL to open:");
    if (newUrl) {
      OpenURL(newUrl);
    }
  };

  const getStatusText = () => {
    if (status.connected) {
      return "🟢 Online";
    } else {
      return `🔴 Offline${status.last_err ? `: ${status.last_err}` : ""}`;
    }
  };

  const getStatusColor = () => {
    return status.connected ? "#4CAF50" : "#f44336";
  };

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column" }}>
      {/* Status Bar */}
      <div
        style={{
          padding: "8px 12px",
          fontSize: "13px",
          borderBottom: "1px solid #e0e0e0",
          backgroundColor: "#f5f5f5",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          fontFamily: "system-ui, -apple-system, sans-serif"
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
          <span style={{ fontWeight: "600" }}>Device Agent</span>
          <span style={{ color: getStatusColor(), fontWeight: "500" }}>
            {getStatusText()}
          </span>
        </div>
        <div style={{ display: "flex", gap: "8px" }}>
          <button
            onClick={handleOpenURL}
            style={{
              padding: "4px 8px",
              fontSize: "12px",
              border: "1px solid #ccc",
              borderRadius: "4px",
              backgroundColor: "white",
              cursor: "pointer"
            }}
          >
            Open URL
          </button>
          <button
            onClick={() => GetStatus().then(setStatus)}
            style={{
              padding: "4px 8px",
              fontSize: "12px",
              border: "1px solid #ccc",
              borderRadius: "4px",
              backgroundColor: "white",
              cursor: "pointer"
            }}
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Content Frame */}
      <div style={{ flex: 1, position: "relative" }}>
        <iframe
          src={url}
          style={{
            width: "100%",
            height: "100%",
            border: "none",
            display: "block"
          }}
          title="Embedded Content"
          sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-top-navigation"
        />

        {/* Loading overlay when disconnected */}
        {!status.connected && (
          <div
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              backgroundColor: "rgba(255, 255, 255, 0.9)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexDirection: "column",
              gap: "16px"
            }}
          >
            <div style={{ fontSize: "18px", color: "#666" }}>
              Connecting to server...
            </div>
            {status.last_err && (
              <div style={{ fontSize: "14px", color: "#f44336", textAlign: "center", maxWidth: "400px" }}>
                {status.last_err}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default App;
