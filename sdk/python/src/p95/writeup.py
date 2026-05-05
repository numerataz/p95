"""Writeup class for creating and publishing research documentation."""

from typing import Any, Dict, List, Optional

from p95.client import P95Client
from p95.config import SDKConfig, get_config


class Writeup:
    """
    A writeup for documenting and sharing ML research.

    Writeups support Markdown with embedded run data using special syntax:
    - {{run:RUN_ID:summary}} - Embed a run summary card
    - {{run:RUN_ID:metrics}} - Embed metrics charts
    - {{run:RUN_ID:config}} - Embed run configuration
    - {{run:RUN_ID:system-info}} - Embed system information
    - {{compare:RUN_ID1,RUN_ID2:metrics:loss}} - Compare metrics across runs

    Example:
        from p95 import Writeup

        # Create and publish a writeup
        writeup = Writeup(
            project="my-team/my-app",
            title="GPT-2 Fine-tuning Results",
        )

        writeup.content = '''
        # Fine-tuning GPT-2 on Custom Dataset

        ## Summary
        We fine-tuned GPT-2 on our custom dataset and achieved significant improvements.

        ## Results
        {{run:abc123:summary}}

        ## Training Metrics
        {{run:abc123:metrics}}

        ## Configuration
        {{run:abc123:config}}
        '''

        writeup.save()
        writeup.publish()
        print(f"Published at: {writeup.url}")
    """

    def __init__(
        self,
        project: str,
        title: str,
        content: str = "",
        writeup_id: Optional[str] = None,
        config: Optional[SDKConfig] = None,
        api_key: Optional[str] = None,
    ):
        """
        Initialize a writeup.

        Args:
            project: Project in format "team-slug/app-slug"
            title: Writeup title
            content: Initial markdown content
            writeup_id: Existing writeup ID (for loading/updating)
            config: SDK configuration
            api_key: API key (overrides config)
        """
        self._config = config or get_config()
        self._client = P95Client(self._config, api_key=api_key)

        # Parse project
        parts = project.split("/")
        if len(parts) != 2:
            raise ValueError("Project must be in format 'team-slug/app-slug'")
        self._team_slug = parts[0]
        self._app_slug = parts[1]

        self._id: Optional[str] = writeup_id
        self._title = title
        self._content = content
        self._is_published = False
        self._share_slug: Optional[str] = None
        self._created = False

        # If writeup_id provided, load existing writeup
        if writeup_id:
            self._load()

    def _load(self) -> None:
        """Load writeup data from the API."""
        if not self._id:
            return

        data = self._client.get_writeup(
            self._team_slug,
            self._app_slug,
            self._id,
        )
        self._title = data.get("title", self._title)
        self._content = data.get("content", self._content)
        self._is_published = data.get("is_published", False)
        self._created = True

    def _create(self) -> None:
        """Create the writeup on the server."""
        if self._created:
            return

        data = self._client.create_writeup(
            self._team_slug,
            self._app_slug,
            self._title,
            self._content,
        )
        self._id = data["id"]
        self._created = True

    @property
    def id(self) -> Optional[str]:
        """The writeup ID."""
        return self._id

    @property
    def title(self) -> str:
        """The writeup title."""
        return self._title

    @title.setter
    def title(self, value: str) -> None:
        """Set the writeup title."""
        self._title = value

    @property
    def content(self) -> str:
        """The writeup content (Markdown)."""
        return self._content

    @content.setter
    def content(self, value: str) -> None:
        """Set the writeup content."""
        self._content = value

    @property
    def is_published(self) -> bool:
        """Whether the writeup is published."""
        return self._is_published

    @property
    def url(self) -> Optional[str]:
        """The public URL if published."""
        if not self._is_published or not self._id:
            return None
        base = self._config.base_url.rstrip("/")
        return f"{base}/w/{self._id}"

    def embed_run(self, run_id: str, embed_type: str = "summary") -> str:
        """
        Generate embed syntax for a run.

        Args:
            run_id: The run ID to embed
            embed_type: Type of embed (summary, metrics, config, system-info)

        Returns:
            The embed syntax string
        """
        return f"{{{{run:{run_id}:{embed_type}}}}}"

    def embed_comparison(
        self,
        run_ids: List[str],
        metric_name: str,
    ) -> str:
        """
        Generate embed syntax for comparing runs.

        Args:
            run_ids: List of run IDs to compare
            metric_name: Name of the metric to compare

        Returns:
            The embed syntax string
        """
        ids = ",".join(run_ids)
        return f"{{{{compare:{ids}:metrics:{metric_name}}}}}"

    def save(self) -> "Writeup":
        """
        Save the writeup (create or update).

        Returns:
            Self for chaining
        """
        if not self._created:
            self._create()
        else:
            self._client.update_writeup(
                self._team_slug,
                self._app_slug,
                self._id,
                title=self._title,
                content=self._content,
            )
        return self

    def publish(self) -> "Writeup":
        """
        Publish the writeup to the marketplace.

        Returns:
            Self for chaining
        """
        if not self._created:
            self._create()

        data = self._client.publish_writeup(
            self._team_slug,
            self._app_slug,
            self._id,
        )
        self._is_published = True
        self._share_slug = data.get("share_slug")
        return self

    def unpublish(self) -> "Writeup":
        """
        Unpublish the writeup (remove from marketplace).

        Returns:
            Self for chaining
        """
        if not self._id:
            return self

        self._client.unpublish_writeup(
            self._team_slug,
            self._app_slug,
            self._id,
        )
        self._is_published = False
        return self


def create_writeup(
    project: str,
    title: str,
    content: str = "",
    publish: bool = False,
    api_key: Optional[str] = None,
) -> Writeup:
    """
    Create a new writeup.

    Args:
        project: Project in format "team-slug/app-slug"
        title: Writeup title
        content: Markdown content
        publish: Whether to publish immediately
        api_key: API key (optional)

    Returns:
        The created Writeup
    """
    writeup = Writeup(
        project=project,
        title=title,
        content=content,
        api_key=api_key,
    )
    writeup.save()

    if publish:
        writeup.publish()

    return writeup
