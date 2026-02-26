import { StartUdpListener, StopUdpListener, SendUdpNumber } from "../../../wailsjs/go/main/App.js";
import { EventsOn } from "../../../wailsjs/runtime/runtime.js";

class UdpState {
    currentNumb = $state(/** @type {number | null} */ (0));
    currentMsg = $state(null);
    isListening = $state(false);
}

export const udpState = new UdpState();

let unlisten = /** @type {(() => void) | null} */ (null);
let listening = false; // internal guard

export const udpStore = {
    startListening: async (port = 8080) => {
        if (listening || udpState.isListening) {
            return `Already listening on port ${port}`;
        }

        try {
            const result = await StartUdpListener(port);
            console.log("[UDP] StartUdpListener result:", result);

            if (typeof result === "string" && result.startsWith("Error")) {
                throw new Error(result);
            }

            if (typeof result === "string" && result.includes("Already listening")) {
                listening = true;
                udpState.isListening = true;
                return result;
            }

            unlisten = EventsOn("udp-message", (message) => {
                console.log("[UDP] event received:", message);
                udpState.currentMsg = message;
                if (message.type === "number") {
                    udpState.currentNumb = message.data.value;
                }
            });

            listening = true;
            udpState.isListening = true;
            return `Listening on port ${port}`;
        } catch (error) {
            if (String(error).includes("Already listening")) {
                listening = true;
                udpState.isListening = true;
                return `Already listening on port ${port}`;
            }

            listening = false;
            udpState.isListening = false;
            throw new Error(`Failed to start: ${error}`);
        }
    },

    stopListening: async () => {
        if (!listening && !udpState.isListening) {
            return "Not listening";
        }

        try {
            if (unlisten) {
                unlisten();
                unlisten = null;
            }
            await StopUdpListener();
            listening = false;
            udpState.isListening = false;

            udpState.currentNumb = null;
            udpState.currentMsg = null;

            return "Stopped listening";
        } catch (error) {
            throw new Error(`Failed to stop: ${error}`);
        }
    },

    sendNumber: async (/** @type {number} */ number, port = 8080) => {
        if (number < 0 || number > 1000000) {
            throw new Error("Number must be between 0-1000000");
        }

        try {
            await SendUdpNumber(number, port);
            return `Sent ${number}`;
        } catch (error) {
            throw new Error(`Failed to send: ${error}`);
        }
    },
};
