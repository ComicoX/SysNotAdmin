document.addEventListener("DOMContentLoaded", function () {
	setInterval(refreshStatuses, 900000);

	document.getElementById("manual-refresh").addEventListener("click", function () {
		refreshStatuses();
	});
});


async function refreshStatuses() {
	try {
		const response = await fetch('/status');
		if (!response.ok) return;

		const data = await response.json();

		data.Local.forEach(service => {
			updateServiceUI("local", service);
		});

		for (const remoteName in data.Remote) {
			data.Remote[remoteName].forEach(service => {
				updateServiceUI(remoteName, service);
			});
		}
	} catch (err) {
		console.error("Failed to refresh statuses:", err);
	}
}

function updateServiceUI(section, service) {
	const statusId = `${section}-${service.Name}-status`;
	const uptimeId = `${section}-${service.Name}-uptime`;

	const statusEl = document.getElementById(statusId);
	const uptimeEl = document.getElementById(uptimeId);

	if (statusEl && uptimeEl) {
		if (service.Active) {
			statusEl.className = "badge green";
			statusEl.textContent = "Running";
		} else if (service.Uptime === "SSH_FAILED") {
			statusEl.className = "badge gray";
			statusEl.textContent = "SSH Failed";
		} else {
			statusEl.className = "badge red";
			statusEl.textContent = "Not Running";
		}
		uptimeEl.textContent = service.Uptime;
	}
}
