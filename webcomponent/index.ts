async function main() {
  const features = await fetch("/features").then((resp) => resp.json());
  console.log("features", features);
  if (features.installable) {
    document.querySelector(".slack-corner")?.classList.remove("hide");
    const newURL = new URL(features.installable);
    newURL.searchParams.set(
      "redirect_uri",
      window.location.origin + "/slack/auth",
    );
    document
      .querySelector("a.slack-corner")
      ?.setAttribute("href", newURL.toString());
  }
}
main().catch(console.error);
