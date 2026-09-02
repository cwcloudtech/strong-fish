import VideoEmbedUrl from "@site/src/components/VideoEmbedUrl";

const VideoPlayer = ({ url }) => {
  return (
    <div 
        style={{
            paddingBottom: "10px"
        }}
    >
        <div
            style={{
                borderRadius: "0.75rem",
                border: "1px solid #e5e5e5",
                padding: "0.75rem",
                boxShadow: "0 4px 12px rgba(0,0,0,0.03)",
                background: "#fff",
                display: "flex",
                flexDirection: "column",
                gap: "0.5rem",
            }}
        >
            <div
                style={{
                    position: "relative",
                    paddingBottom: "56.25%", // 16:9
                    height: 0,
                    overflow: "hidden",
                    borderRadius: "0.5rem",
                }}
            >
                <iframe
                    src={VideoEmbedUrl(url)}
                    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                    allowFullScreen
                    style={{
                        position: "absolute",
                        top: 0,
                        left: 0,
                        width: "100%",
                        height: "100%",
                        border: 0,
                    }}
                />
            </div>
        </div>
    </div>
  );
};

export default VideoPlayer;
