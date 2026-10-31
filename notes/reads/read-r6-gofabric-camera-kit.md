# Read map R6 — go-fabric v0.15.5 camera kit (Sonnet read, 2026-10-31; a map: file:line for every fact, no judgement; verify before acting)

Source: /Users/markus/Documents/GitHub/go-fabric, `git describe --tags` printed `v0.15.5`. Nothing modified. Paths below are relative to that repo.

## Q1. Assembling a Camera device as a host

### Constructor and type
- `NewCamera(opts Options, decls ...Declaration) (*Camera, error)` = `FromSpec(Spec{Type: CameraID, Options: opts, Declarations: decls})`; returns a `*Camera` embedding `*Device` (device/camera_gen.go:89-96). `CameraID = 0x0142`, revision 1 (camera_gen.go:20-24). Type has `Classification: "simple", Composition: schema.CompositionTree` (camera_gen.go:33).
- Typed accessors: `CameraAvStreamManagement()` (:99), `WebRtcTransportProvider()` (:104), `WebRtcTransportRequestor()` (:109, "nil until a declaration mounts it"), `PushAvStreamTransport()`, `CameraAvSettingsUserLevelManagement()`, `ZoneManagement()`, `OccupancySensing()`, all at camera_gen.go:98-131. No `Identify()` accessor in the range read (lines 98-131) — Identify is in Servers list (:67-70) but an accessor was not found there.

### Server clusters (device/camera_gen.go:34-71)
| Cluster | ID | Conformance | Mandatory flag | Features | line |
| --- | --- | --- | --- | --- | --- |
| CameraAvStreamManagement | 0x0551 | M | true | `Features: 0x7`, names VDO, ADO, SNP | :36-40 |
| WebRtcTransportProvider | 0x0553 | M | true | none | :41-45 |
| WebRtcTransportRequestor | 0x0554 | O | | | :46-49 |
| PushAvStreamTransport | 0x0555 | O | | | :50-53 |
| CameraAvSettingsUserLevelManagement | 0x0552 | O | | | :54-57 |
| ZoneManagement | 0x0550 | O | | `Features: 0x1`, TWODCART | :58-62 |
| OccupancySensing | 0x0406 | O | | | :63-66 |
| Identify | 0x0003 | O | | | :67-70 |

Client clusters (camera_gen.go:72-80): WebRtcTransportRequestor 0x0554 `M`, `Mandatory: true`; WebRtcTransportProvider 0x0553 `O`.

### How the ports reach the servers (device/defaults_camera.go)
- `init` registers factories in `selfHeldDefaults` for requestor, chime, avsettings, `csdef.ClusterID: newAvStreamServer`, zones, `wpdef.ClusterID: newProviderServer`, push (defaults_camera.go:42-54).
- `newAvStreamServer` (defaults_camera.go:110-125): `d, _ := b.ports.(csdef.Declare)`; builds `camera.AvStreamConfig{Features: camera.AvStreamFeature(b.opts.Features), Attributes: b.opts.Attributes, Video: d.VideoStreamPort, Audio: d.AudioStreamPort, Snapshot: d.SnapshotStreamPort, Capture: d.SnapshotCapturePort, DataVersion: ...}` (:112-116); `configFrom(...)`, `cfg.Initial = maps.Clone(b.initial)` (:117-119); on `camera.ErrAvStreamConfig` or `camera.ErrNoStreamPort` returns `notServed(err)` (:121-123).
- `newProviderServer` (defaults_camera.go:147-157): `d, _ := b.ports.(wpdef.Declare)`; if `d.ProviderPort == nil` returns `notServed(fmt.Errorf("webrtc transport provider: %w", errNoPort))` (:149-151); else `camera.ProviderConfig{Port: d.ProviderPort, DataVersion: ...}`; `cfg.Streams = av` when `b.sibling(csdef.ClusterID)` is a `*camera.AvStream` (:153-155); `camera.NewProvider(cfg)`.
- `camera.NewAvStream` returns `ErrNoStreamPort` when VDO set and `Video == nil`, ADO and `Audio == nil`, or SNP and (`Snapshot == nil || Capture == nil`) (cluster/camera/avstream.go:194-198; error text :86-90 region, var at cluster/camera/avstream.go ~83-87 "VDO needs a VideoStreamPort, ADO an AudioStreamPort, SNP a SnapshotStreamPort and a SnapshotCapturePort").
- `errNoPort` = "device: the declaration carries no host port for the cluster's server" (device/defaults_ports.go:40). `notServed(err)` wraps `ErrSelectionNotServed` (device/defaults.go:101).

### What notServed/errNoPort means for assembly (device/device.go:380-405, defaults.go:87-94)
- `ErrSelectionNotServed` doc (defaults.go:87-94): "A cluster the host declared — a Declare with a port, or with features the kit's server takes without one — fails the device's construction with it … For the device type's default selection the cluster keeps the generated server and the kit logs one Info line (device.default_server.generated)."
- Code (device.go:390-404): `!errors.Is(err, ErrSelectionNotServed)` → `ErrConfiguration` failure; `case s.declared[r.ID] && (hasPorts(s.ports[r.ID]) || !errors.Is(err, errNoPort))` → returns error (device construction fails); otherwise logs `device.default_server.generated` and keeps the generated `spec.Server` (device.go:399-404).
- `hasPorts` = any non-nil interface field of the Declare other than `HostServer` (device/options.go:229-241).
- Consequence as read: no Declare, or a Declare without any port, for a mandatory cluster → device builds with the generated server (commands unimplemented). A Declare with any port but missing a needed one (e.g. AvStream VDO with only `AudioStreamPort`) → `NewCamera` returns an error. A WebRtcTransportProvider Declare with `ProviderPort == nil` and no other port → generated server kept (declared flag set but `hasPorts` false and err is errNoPort).
- Test evidence: device/camera_streams_declaration_test.go:63-70 (`NewCamera` without declarations: `Server(csdef.ClusterID)` is `*spec.Server`); device/camera_transport_declaration_test.go:130-137 (bare camera has no `*camera.Provider`).

### Declare structs (verbatim fields)
`cluster/spec/webrtctransportprovider/server_gen.go:232-269` (doc preamble lines 228-231 not reproduced):
```go
type Declare struct {
	// Initial is the state the server starts with, over the device
	// type's defaults; an optional attribute it sets is served.
	Initial Patch
	// Features is the FeatureMap selection; zero is the device type's.
	Features Feature
	// Commands are optional commands the server accepts beyond the
	// ones the selection mandates: the kit's server carries them out
	// (through its port), a Handle field below handles one in its place.
	Commands []uint32
	// OnWrite vetoes a controller's write before it is stored
	// (Server.OnWrite); it runs inline.
	OnWrite func(ctx context.Context, next State, attrID uint32) error
	// HandleSolicitOffer ... (SolicitOfferRequest) (SolicitOfferResponse, error)
	HandleSolicitOffer func(ctx context.Context, req SolicitOfferRequest) (SolicitOfferResponse, error)
	HandleProvideOffer func(ctx context.Context, req ProvideOfferRequest) (ProvideOfferResponse, error)
	HandleProvideAnswer func(ctx context.Context, req ProvideAnswerRequest) (struct{}, error)
	HandleProvideIceCandidates func(ctx context.Context, req ProvideIceCandidatesRequest) (struct{}, error)
	HandleEndSession func(ctx context.Context, req EndSessionRequest) (struct{}, error)
	// ProviderPort is the host port the kit's server runs on ([ProviderPort]).
	ProviderPort ProviderPort
	// HostServer is the host's own server for the cluster, mounted in
	// place of the kit's — also for a cluster the device type does not
	// list. The hooks above go on it; Initial, Features and the ports are
	// the kit's server's and are refused beside it.
	HostServer contract.ClusterServer
}
```
(Each Handle field carries the doc "handles the X command in place of the server (Server.HandleX), inline; an optional command it handles is accepted." at server_gen.go:243-257; abbreviated here.)

`cluster/spec/cameraavstreammanagement/server_gen.go:634-687` — same preamble fields (`Initial Patch`, `Features Feature`, `Commands []uint32`, `OnWrite`) then ten Handle fields (`HandleAudioStreamAllocate`, `HandleAudioStreamDeallocate`, `HandleVideoStreamAllocate`, `HandleVideoStreamModify`, `HandleVideoStreamDeallocate`, `HandleSnapshotStreamAllocate`, `HandleSnapshotStreamModify`, `HandleSnapshotStreamDeallocate`, `HandleSetStreamPriorities`, `HandleCaptureSnapshot`), then:
```go
	// VideoStreamPort is the host port the kit's server runs on ([VideoStreamPort]).
	VideoStreamPort VideoStreamPort
	// AudioStreamPort is the host port the kit's server runs on ([AudioStreamPort]).
	AudioStreamPort AudioStreamPort
	// SnapshotStreamPort is the host port the kit's server runs on ([SnapshotStreamPort]).
	SnapshotStreamPort SnapshotStreamPort
	// SnapshotCapturePort is the host port the kit's server runs on ([SnapshotCapturePort]).
	SnapshotCapturePort SnapshotCapturePort
	// HostServer is the host's own server ... (same text as above)
	HostServer contract.ClusterServer
```
(server_gen.go ~677-690.) Declare doc preamble: "every field is optional — the zero value mounts the cluster (an optional one too) with the kit's default server and the device type's defaults. A device takes one Declare per cluster." (server_gen.go:628-633).

### Port interfaces (cluster/spec/cameraavstreammanagement/server_gen.go)
`VideoStreamPort` (doc :572-582; interface :583-588):
> "VideoStreamPort is the camera's video encoder (VDO): the kit's server (cluster/camera AvStream) validates a request against the cluster's rules before it calls VideoStreamAllocate with the requested stream (its id and reference count ignored), which allocates or reuses a stream and returns its id; the server then narrows a reused stream to the intersection of both ranges and calls VideoStreamAllocated with the stream as allocated, which is where the host starts it. VideoStreamModify and VideoStreamDeallocate act on a stream the server knows. An error answers its status, FAILURE without one (after connectedhomeip's CameraAVStreamManagementDelegate)."
```go
type VideoStreamPort interface {
	VideoStreamAllocate(ctx context.Context, args VideoStreamStruct) (videoStreamID uint16, err error)
	VideoStreamAllocated(ctx context.Context, stream VideoStreamStruct, action StreamAllocationAction)
	VideoStreamModify(ctx context.Context, videoStreamID uint16, watermarkEnabled, osdEnabled *bool) error
	VideoStreamDeallocate(ctx context.Context, videoStreamID uint16) error
}
```
`StreamAllocationAction`: `StreamNewAllocation` ("a fresh stream: start it"), `StreamModification` ("a reused stream whose ranges narrowed: restart it if it runs"), `StreamReuse` ("a reused stream left as it was: nothing to do") (server_gen.go ~556-570).

`AudioStreamPort` (doc: "AudioStreamAllocate allocates or reuses a stream for a request the kit's server validated (its id and reference count ignored) and returns its id; AudioStreamDeallocate releases a stream the server knows. An error answers its status, FAILURE without one."):
```go
type AudioStreamPort interface {
	AudioStreamAllocate(ctx context.Context, args AudioStreamStruct) (audioStreamID uint16, err error)
	AudioStreamDeallocate(ctx context.Context, audioStreamID uint16) error
}
```
`SnapshotStreamPort` (doc: "allocates or reuses a stream for a request the kit's server validated and matched against SnapshotCapabilities (FrameRate is the requested MaxFrameRate, EncodedPixels and HardwareEncoder the matching capability's; the id and reference count ignored) and returns its id; SnapshotStreamModify and SnapshotStreamDeallocate act on a stream the server knows. An error answers its status, FAILURE without one."):
```go
type SnapshotStreamPort interface {
	SnapshotStreamAllocate(ctx context.Context, args SnapshotStreamStruct) (snapshotStreamID uint16, err error)
	SnapshotStreamModify(ctx context.Context, snapshotStreamID uint16, watermarkEnabled, osdEnabled *bool) error
	SnapshotStreamDeallocate(ctx context.Context, snapshotStreamID uint16) error
}
```
`SnapshotCapturePort` (doc: "takes a picture (SNP): CaptureSnapshot is called once the kit's server checked that a snapshot stream exists (the named one, when snapshotStreamID is not nil), the resolution and the privacy modes, and returns the image; an image larger than the server's bound answers RESOURCE_EXHAUSTED, an error its status, FAILURE without one."):
```go
type SnapshotCapturePort interface {
	CaptureSnapshot(ctx context.Context, snapshotStreamID *uint16, resolution VideoResolutionStruct) (CaptureSnapshotResponse, error)
}
```
(All four: cluster/spec/cameraavstreammanagement/server_gen.go:~583-632; the Audio/Snapshot ones end at :598 and :615/:627 region — exact start lines not individually recorded; grep `type .*Port interface` in that file.)

`ProviderPort` (cluster/spec/webrtctransportprovider/server_gen.go, doc ends :218, interface :219-225; also `OfferArgs` :190-203 region):
> "ProviderPort is the camera's WebRTC stack (cluster/camera Provider, after connectedhomeip's WebRTCTransportProvider Delegate): the kit's server validates each command — its constraints, the privacy modes, the stream usage and the streams against the camera's CameraAvStreamManagement, the requestor's session — before it calls the port, and keeps CurrentSessions. SolicitOffer and ProvideOffer return the session to store; deferredOffer reports a camera in standby that sends its offer later. The host counts the streams a session uses (the AvStream's reference counts). An error answers its status, FAILURE without one; EndSession's ends the session too."
```go
type ProviderPort interface {
	SolicitOffer(ctx context.Context, args OfferArgs) (session WebRTCSessionStruct, deferredOffer bool, err error)
	ProvideOffer(ctx context.Context, args OfferArgs) (session WebRTCSessionStruct, err error)
	ProvideAnswer(ctx context.Context, sessionID uint16, sdp string) error
	ProvideIceCandidates(ctx context.Context, sessionID uint16, candidates []ICECandidateStruct) error
	EndSession(ctx context.Context, sessionID uint16, reason uint8) error
}
```
`OfferArgs` fields: SessionID uint16, StreamUsage StreamUsageEnum, VideoStreams *[]uint16, AudioStreams *[]uint16, IceServers *[]ICEServerStruct, IceTransportPolicy *string, MetadataEnabled *bool, PeerNodeID uint64, FabricIndex uint8, OriginatingEndpointID uint16, Sdp string (webrtctransportprovider/server_gen.go, struct directly above ProviderPort doc; doc says Sdp is for ProvideOffer).

### Complete assembly found in go-fabric tests
1. device/camera_transport_declaration_test.go:98-113 (`TestCameraServesTheKitTransports`):
```go
usages := []csdef.StreamUsageEnum{csdef.StreamUsageLiveView}
maxZones, sensMax := uint8(4), uint8(5)
d, err := device.NewCamera(device.Options{StableKey: device.StringKey("cam-transports"), Name: "cam"},
	csdef.Declare{
		Initial:         csdef.Patch{SupportedStreamUsages: &usages},
		VideoStreamPort: p, AudioStreamPort: p, SnapshotStreamPort: p, SnapshotCapturePort: p,
	},
	zmdef.Declare{Initial: zmdef.Patch{MaxZones: &maxZones, SensitivityMax: &sensMax}, ZonePort: tp},
	wpdef.Declare{ProviderPort: tp},
	padef.Declare{PushTransportPort: tp},
)
```
(`p := streamPorts{}`, `tp := transportPorts{}` at :100-101; `streamPorts` defined in device/camera_streams_declaration_test.go ~:20-45, returns zero values.)
2. device/camera_streams_declaration_test.go:49-53: `csdef.Declare{Initial: csdef.Patch{SupportedStreamUsages: &usages}, VideoStreamPort: p, AudioStreamPort: p, SnapshotStreamPort: p, SnapshotCapturePort: p}`.
3. node/webrtc_provider_peers_test.go:86: `device.NewCamera(device.Options{StableKey: endpoint.StringKey("cam:1"), Name: "cam"}, wpdef.Declare{ProviderPort: cameraStack{}})` — no AvStream declaration (AvStream stays the generated server per the rule above).
4. Intercom: `New(&IntercomType, ..., csdef.Declare{AudioStreamPort: fakeAudio{}})` builds `*camera.AvStream` (device/defaults_camera_test.go:116-118); IntercomType's AvStream requirement has `Features: 0x1` ADO only (device/intercom_gen.go:35).
- None of these tests set `csdef.Declare.Features` on a Camera; a video-only or audio-less Camera declaration: not found in tests (see Q2).

## Q2. The AvStream server's requirements (cluster/camera/avstream.go)

### AvStreamConfig (avstream.go:111-143)
| Field | Type | Doc (verbatim/short) |
| --- | --- | --- |
| Features | AvStreamFeature | none |
| Attributes | []uint32 | "the optional attributes served, among HardPrivacyModeOn, NightVisionIllum, MicrophoneAgcEnabled, ImageRotation, ImageFlipHorizontal, ImageFlipVertical, StatusLightEnabled and StatusLightBrightness" (:113-117) |
| Initial | map[uint32]any | "the values the server starts with — the capabilities and limits (MaxConcurrentEncoders … MaxNetworkBandwidth, SupportedStreamUsages) and the settings the host persisted — set as AvStream.SetAttributes sets them" (:118-122) |
| StreamUsagePriorities | []StreamUsage | "ranked list of stream usages; nil starts with SupportedStreamUsages, as chip's InitArguments pass the same list for both" (:123-126) |
| AllocatedVideoStreams / AllocatedAudioStreams / AllocatedSnapshotStreams | []VideoStream / []AudioStream / []SnapshotStream | "The streams the host persisted; their reference counts start at 0 (chip LoadAllocatedStreams …:1834-1838)" (:127-132); NewAvStream zeroes ReferenceCount (avstream.go:~232-247) |
| Video / Audio / Snapshot / Capture | VideoStreamPort / AudioStreamPort / SnapshotStreamPort / SnapshotCapturePort | "VDO needs Video, ADO Audio, SNP Snapshot and Capture" (:133-137) |
| MaxSnapshotSize | int | "bounds a captured image; 0 is DefaultMaxSnapshotSize" (:138-140) |
| DataVersion | *cluster.DataVersionTracker | optional host-owned tracker (:141-142) |
- The device kit passes the declaration's `Initial` as `cfg.Initial`; the device-kit path sets `Features`, `Attributes`, ports, `DataVersion` and `Initial` only (defaults_camera.go:112-119). Whether `configFrom` (device) fills `StreamUsagePriorities`/`Allocated*Streams` from `Initial` is not read here (the test at device/camera_streams_declaration_test.go:58-62 shows priorities derived from `SupportedStreamUsages`).
- `DefaultMaxSnapshotSize = int(sessionparams.DefaultMaxTCPMessageSize) - 100` (avstream.go:~75-76; doc says matter.js's 64000 fallback).

### Attributes and their feature/conformance (cluster/spec/cameraavstreammanagement/definition_gen.go:28-50)
- `M` (always): MaxContentBufferSize 0x0006 uint32 (:36), MaxNetworkBandwidth 0x000B uint32 (:41), SupportedStreamUsages 0x000E list[StreamUsageEnum] (:44), StreamUsagePriorities 0x0012 (:47).
- `VDO | SNP`: MaxConcurrentEncoders 0x0000 uint8 (:30), MaxEncodedPixelRate 0x0001 uint32 (:31).
- `VDO`: VideoSensorParams 0x0002 (:32), MinViewportResolution 0x0004 (:34), RateDistortionTradeOffPoints 0x0005 list (:35), CurrentFrameRate 0x000C, AllocatedVideoStreams 0x000F.
- `ADO`: MicrophoneCapabilities 0x0007 (:37), AllocatedAudioStreams 0x0010. `SNP`: SnapshotCapabilities 0x000A (:40), AllocatedSnapshotStreams 0x0011. `SPKR`: SpeakerCapabilities 0x0008, TwoWayTalkSupport 0x0009.
- (Attribute list in definition_gen.go lines 28-50 stops at 0x0014; lines beyond 50 not read.)
- What the server does if the host omits a value: `chipDefaults()` (avstream.go:262-306) sets only attributes whose definition has no default and that are scalars/enums/etc. (MaxConcurrentEncoders 0, MaxEncodedPixelRate 0, MaxContentBufferSize 0, MaxNetworkBandwidth 0, TwoWayTalkSupport NotSupported, Viewport zero struct, …). It does NOT set VideoSensorParams, MinViewportResolution, RateDistortionTradeOffPoints, SupportedStreamUsages, SnapshotCapabilities, MicrophoneCapabilities, SpeakerCapabilities. Whether reading them unset errors or is skipped: not verified.
- Config validation `checkAvStreamConfig` (avstream.go:310-341), chip :85-153: at least one of VDO/ADO/SNP; ICTL/WMARK/OSD/HDR need VDO or SNP; SPKR needs ADO; NightVisionIllum needs NV; MicrophoneAgcEnabled needs ADO; ImageRotation/flips need ICTL and ICTL needs one of them; unknown optional attributes refused. Failure → `ErrAvStreamConfig` → `notServed` (defaults_camera.go:121-123).
- `checkSelection` requires the server's FeatureMap to equal the selection (device/defaults.go:547-552).

### Features / audio-less camera
- Camera type's AvStream requirement is `Features: 0x7` VDO|ADO|SNP (camera_gen.go:38). `NewAvStream` accepts any non-empty subset of VDO/ADO/SNP at construction (avstream.go:324-325 requires at least one; :194-198 requires only the ports of the set features).
- device/device.go:431-433 compares `r.Features | clusterMandatedFeatures(...)` with the selection `sel` and reports `missing := r.Features &^ sel` (context lines not read). Whether `csdef.Declare{Features: VDO|SNP}` is accepted on a Camera (dropping ADO) was NOT verified; no test found that does it. Closest evidence: IntercomType mandates ADO only (intercom_gen.go:35) and builds with only `AudioStreamPort`.
- Feature consts: `FeatureAudio`, `FeatureVideo`, `FeatureSnapshot`, `FeaturePrivacy`, `FeatureSpeaker`, `FeatureImageControl`, `FeatureWatermark`, `FeatureOnScreenDisplay`, `FeatureLocalStorage`, `FeatureHighDynamicRange`, `FeatureNightVision` (avstream.go:~52-64).

### AllocateVideoStream validation (avstream.go:703-760, `videoStreamAllocate`)
In order: payload type (:704-707 INVALID_COMMAND); `optionalFieldsMatch(WatermarkEnabled, OsdEnabled)` — WatermarkEnabled present exactly with WMARK, OSDEnabled exactly with OSD (:708-710, def :688-690, INVALID_COMMAND); CONSTRAINT_ERROR for: StreamUsage not Recording/Analysis/LiveView (`liveStreamUsage`, :669-671, check :717), VideoCodec > Av1 (:719), `MinFrameRate < 1 || MinFrameRate > MaxFrameRate` (:721), any resolution dimension < 1 or Min > Max (:723-726), `MinBitRate < 1 || MinBitRate > MaxBitRate` (:727), `KeyFrameInterval > 65500` (:729). Then `!a.prioritised(req.StreamUsage)` → INVALID_IN_STATE (:732-734). Then `a.video.VideoStreamAllocate(ctx, args)` (:746) with the new id either appended (:751-754) or, if the id exists, narrowed to the intersection (`narrowVideoStream` :775-790) with action `StreamReuse`/`StreamModification`; publishes AllocatedVideoStreams when changed; calls `VideoStreamAllocated(ctx, stream, action)` (:766-769). Port errors map via `portError("VideoStreamAllocate", err)`.
- The server does NOT itself check MaxEncodedPixelRate/MaxConcurrentEncoders/RateDistortion/sensor limits in `videoStreamAllocate`; `AvStream.ResourceAvailable(candidateEncodedPixelRate uint32, encoderRequired bool) bool` is offered "for a port to decide RESOURCE_EXHAUSTED" (avstream.go:648-667). Also `ReusableVideoStreamID(req)`/`ReusableSnapshotStreamID(req)` are host methods (webrtc/streams Host interface, cameraavstreammanagement/server_gen.go:~540-550).

### Snapshots
- `snapshotStreamAllocate` (avstream.go:1010-1063): CONSTRAINT_ERROR for ImageCodec > Heic, MaxFrameRate == 0, bad resolutions, Quality outside 1..100; then `matchingSnapshotCapability(SnapshotCapabilities, codec, frameRate, minRes, maxRes)` (:975-987): first entry with equal `ImageCodec`, `MaxFrameRate >= requested`, and `Resolution` within [Min,Max] of the request; none → DYNAMIC_CONSTRAINT_ERROR (:1019 area). `EncodedPixels` = capability `RequiresEncodedPixels`; `HardwareEncoder` = `*RequiresHardwareEncoder` when set (SnapshotStream passed to port). `SnapshotCapabilitiesStruct` fields: Resolution, MaxFrameRate (min 1), ImageCodec, RequiresEncodedPixels, RequiresHardwareEncoder* (definition_gen.go:391-397).
- `CaptureSnapshotResponse` = `{Data []byte; ImageCodec ImageCodecEnum; Resolution VideoResolutionStruct}` (definition_gen.go:1054-1058). `captureSnapshot` (avstream.go:1183-1217): NOT_FOUND when no snapshot stream allocated or the named one unknown; CONSTRAINT_ERROR for RequestedResolution width or height < 1; INVALID_IN_STATE when SoftLivestreamPrivacyModeEnabled or HardPrivacyModeOn; calls `a.capture.CaptureSnapshot(ctx, streamID, req.RequestedResolution)`; `len(image.Data) > a.maxImage` → RESOURCE_EXHAUSTED (:1213-1215). The server does not check the returned ImageCodec/Resolution against the request; no resampling or format check found in this function. The port must deliver the bytes, codec and resolution itself (no further rule found).
- Image-format enum values (`ImageCodecEnum`): not read (avstream.go uses `csdef.ImageCodecHeic` as the highest at :~1000).

## Q3. The provider's signals (v0.15.5)

### cluster/camera/provider_peer.go (150 lines, read in full)
- `type ProviderICECandidate = wpdef.ICECandidateStruct` (:31).
- `PeerConnector interface { ConnectPeer(ctx, fabricIndex uint8, peerNodeID uint64) (PeerSession, error) }` (:37-39). Doc: node attaches its bridge's `bridge.Bridge.ConnectPeer` to every Provider it assembles (:33-36).
- `PeerSession interface { Invoke(ctx, endpoint uint16, clusterID, commandID uint32, fields spec.Encodable) (*spec.Node, error) }` (:45-47); "nil for a status-only Success; another status is an error carrying it (im.StatusCodeError)" (:41-44).
- Errors (:50-62): `ErrNoPeerConnector` ("camera: WebRtcTransportProvider has no PeerConnector"), `ErrUnknownSession` ("camera: no WebRTC session of the id"), `ErrNoICECandidates` ("camera: no ICE candidates to send").
- `AttachPeerConnector(c PeerConnector)` — "nil detaches it. The node attaches its own at Start and after each assembly; a host that runs the server outside a node attaches one itself." (:64-72).
- `Offer(ctx, sessionID uint16, sdp string) error` (:78-80) → requestor `CmdOffer` with `OfferRequest{WebRtcSessionId, Sdp}`.
- `Answer(ctx, sessionID uint16, sdp string) error` (:84-86) → `CmdAnswer`.
- `IceCandidates(ctx, sessionID uint16, candidates []ProviderICECandidate) error` (:93-106): empty → `ErrNoICECandidates` not sent; empty `SdpMid` becomes null.
- `End(ctx, sessionID uint16, reason uint8) error` (:113-120): signals `CmdEnd`; unless the error is `ErrNoPeerConnector` or `ErrUnknownSession`, removes the session from CurrentSessions "whether or not the command reached the peer"; returns the command's error.
- `signal` (:124-150): finds the session in `p.sessions` by Id; `peers == nil` → `ErrNoPeerConnector`; not found → `ErrUnknownSession`; `peers.ConnectPeer(ctx, session.FabricIndex, session.PeerNodeId)`; `sess.Invoke(ctx, session.PeerEndpointId, wrdef.ClusterID, commandID, fields)`.

### cluster/camera/provider.go
- `ProviderClusterID` (:~24); type aliases `ProviderSession`, `ICEServer`, `ProviderPort`, `OfferArgs` (:~26-34); `MaxSessionID uint16 = 65534` (:38); `ErrNoProviderPort` (:42).
- `ProviderConfig` (:45-): `Port ProviderPort` "camera's WebRTC stack; required"; `Streams StreamSource` "the camera's CameraAvStreamManagement server; nil is a camera without streams, whose every offer fails the stream checks"; `UTCTimeKnown func() bool` "whether the node's UTCTime is not null — a turns: or stuns: ICE server needs it. nil reports known"; `DataVersion *cluster.DataVersionTracker`.
- `NewProvider(cfg ProviderConfig) (*Provider, error)` (:104): `cfg.Port == nil` → `ErrNoProviderPort`; handlers for the five commands; CurrentSessions starts empty.
- `Sessions() []ProviderSession` (:128-132; "CurrentSessions, in the order the sessions were added"). There is no method named `CurrentSessions` on Provider; the attribute is `wpdef.AttrCurrentSessions`.
- `UpsertSession(session ProviderSession) error` (:136-147): replaces in place by `Id`, else appends; stores with origin `OriginHostSet`.
- `RemoveSession(sessionID uint16)` (:150-152): only a removal reports a change.
- Both are also the interface `wpdef.Host` (cluster/spec/webrtctransportprovider/server_gen.go, `UpsertSession`, `RemoveSession`; reached via `Server.Host()`).

### docs/guides/camera.md (signalling model, lines 38-130)
- :38-60 requestor model: "A camera controller (the requestor) and a camera (the provider) set up a WebRTC stream … After that the camera signals the session on the controller's WebRtcTransportRequestor" (steps 1-6 :43-67).
- :80-83 ACL: "The node installs the access the camera needs. At `Start` and after each commissioning, on the first fabric, it adds one Operate entry for CASE with no subject, targeting the requestor's endpoint and cluster."
- "The camera's signals" (:90-130): "Once a controller has opened a session with `SolicitOffer` or `ProvideOffer` and the `ProviderPort` has returned it, the host signals the controller's WebRtcTransportRequestor through `camera.Provider`" (:93-96); the four methods (:98-111); "Each signal is one `Invoke` of the requestor's command on the session's peer node and fabric, at the endpoint the session recorded as `PeerEndpointId` (the `OriginatingEndpointID` of the offer), over a CASE session the node initiates itself or the newest live one with that peer." (:113-117); "The node attaches the route (`camera.PeerConnector`) to every provider it assembles, at `Start` and with each assembly, so a camera added to a running node has it before a controller can offer it a session. A provider that runs outside a node answers `camera.ErrNoPeerConnector` until the host attaches a connector with `Provider.AttachPeerConnector`." (:122-127); "The peer's refusal comes back as an error carrying its status (`im.StatusCodeError`): `NOT_FOUND` when the requestor holds no session of that id from this node, `UNSUPPORTED_ACCESS` when its ACL lacks the Operate entry" (:130-134 region); "The signals carry an SDP, so they can exceed one UDP message (1178 bytes of payload). The node initiates CASE over UDP only and reuses the newest live session with the peer, of either transport; a signal that does not fit the session it would go over is refused with `bridge.ErrPayloadTooLarge` and not sent. connectedhomeip establishes a TCP session for them (`TransportPayloadCapability::kLargePayload`); a controller that reached the camera over TCP leaves the session the signals need."
  (Exact line numbers of the last paragraphs: the "camera's signals" section sits within the sed range 38-130 read; per-sentence line numbers beyond those given are approximate — verify with `grep -n`.)

### docs/guides/camera.md (transport model, lines 331-400)
- :331 "Both transports check a request against the camera's stream state, a `camera.StreamSource`. The device's `*camera.AvStream` is one." Null stream id picks the first allocated stream of the usage (WebRTC also another usage's stream when none matches); privacy modes refuse a usage with INVALID_IN_STATE; "The host counts the streams a transport uses with the AvStream's `Update…StreamRefCount`."
- "WebRTC offer and answer": checks (≤10 ICE servers of ≤10 URLs, field lengths, privacy modes, usage in StreamUsagePriorities → DYNAMIC_CONSTRAINT_ERROR, stream lists vs deprecated stream ids not both → INVALID_COMMAND, each stream, turns:/stuns: needs a known UTCTime via `ProviderConfig.UTCTimeKnown`); "It then allocates a session id from 0 and calls the `ProviderPort` with the validated `OfferArgs`. The port returns the session to store in CurrentSessions, and for a solicited offer whether the offer is deferred."; "`EndSession` removes the session whatever the port answers. The host changes a session itself with `Provider.UpsertSession` and `RemoveSession`."

### node/webrtc_provider_peers.go (56 lines, read in full)
- `bridgePeerConnector{br *bridge.Bridge}` implements `camera.PeerConnector` via `c.br.ConnectPeer` (:19-27).
- `camerasOnline(ctx)` = `ensureRequestorACL(ctx)` then `attachCameraPeers()` (:32-35); called from node/node.go:752 (Start).
- `attachCameraPeers()` (:43-56): `topo := n.br.Topology()`; nil → return; iterates `topo.Endpoints`, `endpoint.ClusterServers(ep)`, and for each `*camera.Provider` calls `AttachPeerConnector(connector)`. Doc: "Called at Start and after each assembly: a camera added to a running node gets its connector with the assembly that mounts it, before any controller can offer it a session." (:37-42)
- Second caller: `Aggregator.reassemble` calls `a.n.attachCameraPeers()` after `a.n.br.Reassemble(ctx)` (node/aggregator.go:586-593; call at :592).
- Only providers that are `*camera.Provider` get it: a Declare-less camera (generated server) or a `HostServer` is not attached (type assertion at :51).

### bridge.ErrPayloadTooLarge (bridge/ota_provider_link.go)
- `var ErrPayloadTooLarge = errors.New("bridge: the command does not fit one message of the session")` (:65). Doc (:57-64): "an Invoke whose request does not fit one message of the session it would go over: over UDP [exchangeMaxPayloadSize], over a TCP connection the connection's largest message. connectedhomeip's camera sends its WebRTC signals with TransportPayloadCapability::kLargePayload, which establishes the session over TCP …; this node initiates CASE over UDP only and reuses the newest live session of the peer, so a large payload reaches the peer when the peer came in over TCP."
- Limit constants: `matterMessageOverhead = 26 + 12 + 16` (:51); `exchangeMaxPayloadSize = (1280 - 48) - matterMessageOverhead` (:48) (= 1178 by arithmetic; docs/guides/camera.md says 1178). Per-exchange limit `ex.MaxPayloadSize()` (:155; TCP-aware at :549).
- Raised in `PeerSession.Invoke` (:136-157): `len(payload) > limit` → `fmt.Errorf("%w: command 0x%02x of %d bytes, the session carries %d", ErrPayloadTooLarge, ...)`, before sending.
- Test: node/webrtc_provider_peers_test.go:174-211 (1500-byte SDP over UDP → `bridge.ErrPayloadTooLarge`).

## Q4. Node-side facts for a bridge

- Bridged (aggregator) endpoint accepts a Camera: node/webrtc_provider_peers_test.go:87 and :96 pass `fabrictest.Topology{Bridged: []node.Endpoint{cam}}` (`Bridged` = "the endpoints the node's Aggregator serves", fabrictest/device.go:49-55) and the signalling test passes against it. `Aggregator.Add(ctx, ep Endpoint)` (node/aggregator.go:246) takes any `Endpoint`; no camera-specific code in node/aggregator.go except `attachCameraPeers` at :592. `grep -in camera` in device/aggregator_gen.go, device/bridged_node_gen.go: no hits. No validator rule excluding Camera from a bridged node found in node/aggregator.go or device/device.go by the greps run (device.go:601-612 contains a BridgedNode rule only about PowerSource on parts, "a part is no bridged node"; not camera-related).
- node/composite_devicetype_test.go:29-48 builds `device.NewCamera(opts(key))` as a part of `NewFloodlightCamera` / VideoDoorbell (composite types) — not an aggregator test.
- Not verified: whether a Camera as a Bridged endpoint under an Aggregator with the Camera's Descriptor (device type 0x0142 composition tree) is conformant for real controllers; no live test cited.

### Ecosystem gate, docs/guides/camera.md:414-425 (verbatim)
```
## The ecosystem gate

Concept v10 §D (owner decision, 2026-10-27), verbatim:

> **Ökosystem:** Kamera-Cluster erscheinen in keinem Host-Gerät, bevor ein
> Livetest mit Apple Home (das einzige Ökosystem mit Matter-Kamera-Support
> 2026) bestanden ist — dieselbe Regel wie MSCH.

In English: no host device carries a camera cluster until a live test
with Apple Home has passed. Apple Home is the only ecosystem with Matter
camera support in 2026. The same rule applies to MSCH. The reference
bridge composes none of these clusters.
```
(The heading is at :405 area; the sed range 325-425 ends at this paragraph, file is 425 lines.)
- Other occurrences (grep `ecosystem gate|Apple Home` in docs/, CHANGELOG.md, cmd, examples): docs/matterjs-comparison.md:132 "No host device shows a camera cluster before a live pairing with Apple Home, the owner's hand test (report 27-C §6)."; docs/roadmap.md:272 "hosts serve MSCH behind a flag until the owner's live pairing with Apple Home is verified"; docs/roadmap.md:292 "camera clusters stay out of host devices until a live pairing with Apple Home passes."
- An option flag in go-fabric for the camera gate: not found (no file in examples/ or reference-bridge contains `camera` per grep; `ls cmd` printed nothing). `reference-bridge` directory and `examples/reference-bridge` were not searched individually beyond a failed glob (grep returned "no matches" for reference-bridge/*.go) — treat as not found, unverified.

## Q5. go-fabric's own test harness

### cluster/camera/provider_peer_test.go (273 lines)
- Types: `peerCall` (:25), `fakePeers` (:36; PeerConnector recording every Invoke; fields `calls`, `connectErr`, `invokeErr`), `fakePeerSession` (:43), `statusErr{code im.StatusCode}` (:74-77 implements `MatterStatusCode`).
- Methods: `(*fakePeers).ConnectPeer` (:49), `(*fakePeerSession).Invoke` (:56), `(*fakePeers).last` (:63).
- Helper `signalProvider(t)` (:87-99): `newProvider(t, nil)`, `AttachPeerConnector(&fakePeers{})`, `UpsertSession(camera.ProviderSession{Id: 5, PeerNodeId: sigPeerNode, PeerEndpointId: sigPeerEndpoint, FabricIndex: sigPeerFabric, StreamUsage: wpdef.StreamUsageLiveView})`.
- Tests: `TestProviderSignalsNeedAConnector` (:102), `...UnknownSession` (:119), `...ReachThePeer` (:142), `TestProviderEndRemovesTheSession` (:213), `TestProviderSignalErrors` (:260).
- `newProvider(t, streams, edit...)` and `mockProvider` live in cluster/camera/provider_chip_test.go:88-100 / :~40-85 (`mockProvider.ProvideOffer` at :57).

### node/webrtc_provider_peers_test.go
- Fakes: `cameraStack` (:25-41, a `wpdef.ProviderPort` returning sessions built from `OfferArgs`: SolicitOffer :27, ProvideOffer :31, ProvideAnswer :35, ProvideIceCandidates :37, EndSession :41); `requestorStack` (:45-51 and methods :53-72, a `wrdef.RequestorPort` recording answers, candidates, ended reasons, sessions).
- Tests: `TestCameraProviderSignalsReachTheRequestor` (:83), `TestCameraProviderSignalTooLargeForUDP` (:174); helper `oneFabric` (:214).
- Harness: `quietSite(t)`, `parityCtx(t)`, `site.CommissionedPair(fabrictest.PairOptions{Device: fabrictest.Topology{Bridged: []node.Endpoint{cam}}})`, `site.AddDevice(...)`, `pair.Controller.Commission(ctx, ctlDev, fabrictest.CommissionOptions{})`, `site.Run(ctx, fn)`, `EndpointOf("cam:1")` (:86-110, :130-160). `fabrictest` is a non-test package (fabrictest/device.go, doc.go).

### Exported test helper for camera
- `cluster/camera/cameratest`: directory does not exist. `grep 'func Fake|func Test.*Port|type fake'` over cluster/camera non-test-function lines found only `fakePeers`, `fakePeerSession` (provider_peer_test.go:36,43), `fakeStreams` (provider_rules_test.go:24), `fakeTLS` (pushtransport_chip_test.go:110), `fakeCerts` (:121), `fakeZones` (:134) — all unexported, in `_test.go` files. No exported camera test helper found. Other unexported fakes: device/camera_streams_declaration_test.go `streamPorts` (zero-value ports), device/camera_transport_declaration_test.go `transportPorts` (ZonePort/ProviderPort/PushTransportPort zero implementations), device/defaults_camera_test.go `fakeAudio`, `fakeChime`, `fakePTZ`, `fakeZones`.
- Public harness available outside tests: `fabrictest` package (Topology, Pair, Device, `CommissionedPair`, `Run`) — used by node/webrtc_provider_peers_test.go.

### In-process camera controller
- `device.NewCameraController(opts Options, decls ...Declaration) (*CameraController, error)` (device/camera_controller_gen.go:72-75 doc, :75 signature). "The device kit makes `camera.Requestor` the default of every device that mounts WebRtcTransportRequestor, such as `device.NewCameraController`." (docs/guides/camera.md:24-25). Its mandatory cluster: the type's `Clients` and requestor server; construction used at node/webrtc_provider_peers_test.go:92 with `wrdef.Declare{RequestorPort: stack}`.
- Requestor port: `RequestorPort interface { Offer(ctx, session WebRTCSessionStruct, req OfferRequest); Answer(ctx, session, sdp string); IceCandidates(ctx, session, candidates []ICECandidateStruct); End(ctx, session, reason uint8) }` (cluster/spec/webrtctransportrequestor/server_gen.go:213-218); optional `RequestorRefusals` (:220-). `camera.Requestor` methods: `Sessions()` (:112), `UpsertSession(WebRTCSession)` (:128), `RemoveSession(wrdef.SessionKey)` (:145) (cluster/camera/requestor.go).
- A controller-side API that sends `ProvideOffer`/`SolicitOffer` to a provider: not found in cluster/camera/requestor.go function list (no method sends commands). go-fabric's tests drive the provider by calling `prov.MatterInvoke(ctx, wpdef.CmdSolicitOffer, wpdef.SolicitOfferRequest{...})` in-process with a context from `im.WithSubject(im.WithFabricFilter(context.Background(), false, 1), 0x77, nil)` (device/camera_transport_declaration_test.go:77-79, call :122). Tests that send `ProvideOffer` over the wire from `pair.Controller` to a camera: not found (grep `CmdProvideOffer|ProvideOfferRequest` in *_test.go hits only cluster/camera/provider_chip_test.go, provider_rules_test.go, the spec definition test; client/interaction tests mention ProvideOffer only as an "L" quality command in comments: client/interaction/client_interaction_invoke_parity_matterjs_test.go:237, large_message_parity_matterjs_test.go:157).

## Summary table of unverified / not found
| Item | Status |
| --- | --- |
| `Declare.Features` subset (audio-less / ADO dropped) accepted on a Camera | not verified; no test found; device/device.go:431-433 feature comparison not read in context |
| What happens when host omits VideoSensorParams, MinViewportResolution, RateDistortionTradeOffPoints, SupportedStreamUsages, SnapshotCapabilities (read errors vs unset) | not verified; `chipDefaults` does not set them (avstream.go:262-306) |
| Image-format enum values and any resolution/codec check on the captured image | not read; `captureSnapshot` checks only size (avstream.go:1213) |
| `configFrom` (device) mapping of Initial onto StreamUsagePriorities/Allocated*Streams | not read |
| Exact start lines of AudioStreamPort / SnapshotStreamPort / SnapshotCapturePort / OfferArgs definitions | approximate (file:~range); grep `type .*Port interface` |
| Line numbers of individual sentences in docs/guides/camera.md "The camera's signals" after :90 | approximate; quoted text is verbatim from the sed range 38-130 |
| Camera on a bridged node under real controllers; any validator excluding camera | no exclusion found by grep; no live evidence |
| Ecosystem-gate option flag in go-fabric (examples, reference-bridge, cmd) | not found; reference-bridge directories not individually grepped |
| Controller-side sender of ProvideOffer/SolicitOffer in go-fabric | not found (only direct `MatterInvoke` in tests) |
| Exported camera test helper (`cameratest`) | not found |
| `Identify()` typed accessor on `*device.Camera` | not found in camera_gen.go:98-131 |
