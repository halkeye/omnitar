(function () {
  let controller = new AbortController();

  window.docReady = function docReady(fn) {
    // see if DOM is already available
    if (
      document.readyState === "complete" ||
      document.readyState === "interactive"
    ) {
      // call on next available tick
      setTimeout(fn, 1);
    } else {
      document.addEventListener("DOMContentLoaded", fn);
    }
  };

  document.addEventListener("click", function (e) {
    if (!e.target) {
      return;
    }

    const dataset = e.target.dataset;
    if (!dataset) {
      return;
    }

    if (dataset.confirm) {
      if (!confirm(dataset.confirm)) {
        e.preventDefault();
        e.stopPropagation();
        return false;
      }
    }

    if (dataset.method) {
      e.preventDefault();
      e.stopPropagation();
      fetch(e.target.href, { method: dataset.method })
        .then((r) => {
          if (!r.ok) {
            throw new Error(r.statusText);
          }
        })
        .then(() => {
          location.reload();
        })
        .catch((e) => {
          alert(e);
        });
      return;
    }
  });

  window.webauthnRegisterStart = async function webauthnRegisterStart(e) {
    e.preventDefault();
    if (
      typeof window.PublicKeyCredential === "undefined" ||
      typeof window.PublicKeyCredential.isConditionalMediationAvailable !==
        "function"
    ) {
      return;
    }

    // Abort ongoing WebAuthn request
    controller.abort();
    controller = new AbortController();

    try {
      const options = PublicKeyCredential.parseCreationOptionsFromJSON(
        await fetch("/webauthn/register")
          .then((r) => r.json())
          .then((options) => ({
            attestation: "direct",
            authenticatorSelection: {
              requireResidentKey: true,
              residentKey: "required",
              userVerification: "preferred",
            },
            ...options.publicKey,
          })),
      );
      console.log(options);
      const cred = await navigator.credentials.create({
        signal: controller.signal,
        publicKey: options,
        mediation: "conditional",
      });
      await fetch("/webauthn/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "same-origin",
        body: JSON.stringify(cred.toJSON()),
      }).then((r) => {
        if (!r.ok) {
          throw new Error(r.statusText);
        }
      });
    } catch (err) {
      console.error("Error:", err);
    }
  };

  window.webauthnSigninStart = async function webauthnSigninStart(e) {
    e.preventDefault();
    if (
      typeof window.PublicKeyCredential === "undefined" ||
      typeof window.PublicKeyCredential.isConditionalMediationAvailable !==
        "function"
    ) {
      return;
    }

    // Abort ongoing WebAuthn request
    controller.abort();
    controller = new AbortController();

    try {
      const options = PublicKeyCredential.parseRequestOptionsFromJSON(
        await fetch("/webauthn/login")
          .then((r) => r.json())
          .then((options) => ({
            allowCredentials: [],
            userVerification: "preferred",
            ...options.publicKey,
          })),
      );

      // Use platform authenticator and discoverable credential
      options.authenticatorSelection = {
        authenticatorAttachment: "platform",
        requireResidentKey: true,
      };

      const cred = await navigator.credentials.get({
        signal: controller.signal,
        mediation: "conditional",
        publicKey: options.publicKey,
      });
      await fetch("/webauthn/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "same-origin",
        body: JSON.stringify(cred.toJSON()),
      }).then((r) => {
        if (!r.ok) {
          throw new Error(r.statusText);
        }
      });
    } catch (err) {
      console.error("Error with conditional UI:", err);
    }
  };
})();
