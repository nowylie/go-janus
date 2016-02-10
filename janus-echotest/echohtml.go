package main

const echoHtml = `
<!DOCTYPE html>
<html><head>
	<style>
		video {
			display: inline-block;
			width: 480px; height: 320px;
			border-radius: 4px;
			background: black;
		}
	</style>
	<script>
		var RTCPeerConnection = window.mozRTCPeerConnection || window.webkitRTCPeerConnection;
		var SessionDescription = window.mozRTCSessionDescription || window.RTCSessionDescription;
		navigator.getUserMedia = navigator.getUserMedia || navigator.mozGetUserMedia || navigator.webkitGetUserMedia;
		
		var errorHandler = function(error) {
			console.error(error);
		}
		
		var Servers = [
		//	{urls: "stun:192.168.1.239"}
		//	{urls: "stun:101.178.170.14"}
		];
	</script>
</head><body>
	<video id="send" autoplay muted></video>
	<video id="recv" autoplay></video>
	<script>
		// Signal channel
		var xhr = new XMLHttpRequest;
		xhr.open("post", "/conf");
		xhr.onload = function() {
			if (xhr.status !== 200) return;
			var answer = JSON.parse(xhr.responseText);
			gotAnswer(answer);	
		}

		var constraints = {audio: true, video: true};
		navigator.getUserMedia(constraints, gotStream, errorHandler);

		// Local PeerConnection setup
		var pc = new RTCPeerConnection({iceServers: Servers});

		var body = {"audio": true, "video": true};
		var jsep = "";
		var candidates = [];

		// Wait for candidates to be gathered, then send Offer
		var iceDone = false;
		pc.onicecandidate = function(ev) {
			if (iceDone) return;
			if (ev.candidate === null) {
				//candidates.push({"completed": true});

				console.log("Sending offer");
				xhr.send(JSON.stringify({"body": body, "offer": jsep, "candidates": candidates}));
				iceDone = true;
				return;
			}
			candidates.push(ev.candidate);
			console.log("Candidate added");
		}
		
		// Remote peer added video
		var recvVideo = document.getElementById("recv");
		pc.onaddstream = function(ev) {
			console.log("Adding remote stream");
			recvVideo.src = URL.createObjectURL(ev.stream);
		}
		
		// Get local video stream
		var sendVideo = document.getElementById("send");
		function gotStream(stream) {
			sendVideo.src = URL.createObjectURL(stream);
			pc.addStream(stream);
			var options = {offerToReceiveAudio: true, offerToReceiveVideo: true};
			pc.createOffer(gotOffer, errorHandler, options);
		}

		function gotOffer(offer) {
			pc.setLocalDescription(offer, function() {
				jsep = offer;
				console.log("Offer added");
			}, errorHandler);
		}

		function gotAnswer(answer) {
			var desc = new SessionDescription(answer);
			pc.setRemoteDescription(desc, function() {
				console.log("Remote description set");
			}, errorHandler);
		}

	</script>
</body></html>
`
