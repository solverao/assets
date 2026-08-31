CREATE TABLE `task` (
	`id` text PRIMARY KEY NOT NULL,
	`title` text NOT NULL,
	`priority` integer DEFAULT 1 NOT NULL
);

CREATE TABLE `account` (
	`id` text PRIMARY KEY NOT NULL,
	`account_id` text NOT NULL,
	`provider_id` text NOT NULL,
	`user_id` text NOT NULL,
	`access_token` text,
	`refresh_token` text,
	`id_token` text,
	`access_token_expires_at` integer,
	`refresh_token_expires_at` integer,
	`scope` text,
	`password` text,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer NOT NULL,
	FOREIGN KEY (`user_id`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE cascade
);

CREATE INDEX `account_userId_idx` ON `account` (`user_id`);
CREATE TABLE `session` (
	`id` text PRIMARY KEY NOT NULL,
	`expires_at` integer NOT NULL,
	`token` text NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer NOT NULL,
	`ip_address` text,
	`user_agent` text,
	`user_id` text NOT NULL,
	FOREIGN KEY (`user_id`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE cascade
);

CREATE UNIQUE INDEX `session_token_unique` ON `session` (`token`);
CREATE INDEX `session_userId_idx` ON `session` (`user_id`);
CREATE TABLE `user` (
	`id` text PRIMARY KEY NOT NULL,
	`name` text NOT NULL,
	`email` text NOT NULL,
	`email_verified` integer DEFAULT false NOT NULL,
	`image` text,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL
);

CREATE UNIQUE INDEX `user_email_unique` ON `user` (`email`);
CREATE TABLE `verification` (
	`id` text PRIMARY KEY NOT NULL,
	`identifier` text NOT NULL,
	`value` text NOT NULL,
	`expires_at` integer NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL
);

CREATE INDEX `verification_identifier_idx` ON `verification` (`identifier`);
CREATE TABLE `categories` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`icon` text,
	`slug` text NOT NULL,
	`parentId` integer,
	`sort_order` integer DEFAULT 0 NOT NULL,
	`is_active` integer DEFAULT true NOT NULL,
	`description` text,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer,
	FOREIGN KEY (`parentId`) REFERENCES `categories`(`id`) ON UPDATE no action ON DELETE no action
);

CREATE UNIQUE INDEX `categories_name_unique` ON `categories` (`name`);
CREATE UNIQUE INDEX `categories_slug_unique` ON `categories` (`slug`);
CREATE TABLE `attribute_values` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`attribute_id` integer NOT NULL,
	`name` text NOT NULL,
	`slug` text NOT NULL,
	`description` text,
	`relevance` integer DEFAULT 1 NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer,
	FOREIGN KEY (`attribute_id`) REFERENCES `attributes`(`id`) ON UPDATE no action ON DELETE cascade
);

CREATE UNIQUE INDEX `attribute_values_name_unique` ON `attribute_values` (`name`);
CREATE UNIQUE INDEX `attribute_values_slug_unique` ON `attribute_values` (`slug`);
CREATE TABLE `attributes` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`slug` text NOT NULL,
	`description` text,
	`relevance` integer DEFAULT 1 NOT NULL,
	`is_required` integer DEFAULT false NOT NULL,
	`is_multiselect` integer DEFAULT true NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer
);

CREATE UNIQUE INDEX `attributes_slug_unique` ON `attributes` (`slug`);
CREATE TABLE `category_attributes` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`category_id` integer NOT NULL,
	`attribute_id` integer NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer,
	FOREIGN KEY (`category_id`) REFERENCES `categories`(`id`) ON UPDATE no action ON DELETE cascade,
	FOREIGN KEY (`attribute_id`) REFERENCES `attributes`(`id`) ON UPDATE no action ON DELETE cascade
);

CREATE UNIQUE INDEX `category_attributes_category_attribute_unique` ON `category_attributes` (`category_id`,`attribute_id`);
CREATE TABLE `asset_attribute_values` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`asset_id` integer NOT NULL,
	`value_id` integer NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer,
	FOREIGN KEY (`asset_id`) REFERENCES `assets`(`id`) ON UPDATE no action ON DELETE cascade,
	FOREIGN KEY (`value_id`) REFERENCES `attribute_values`(`id`) ON UPDATE no action ON DELETE cascade
);

CREATE UNIQUE INDEX `asset_attribute_values_asset_value_unique` ON `asset_attribute_values` (`asset_id`,`value_id`);
CREATE INDEX `asset_attribute_values_value_idx` ON `asset_attribute_values` (`value_id`);
CREATE TABLE `asset_blobs` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`checksum` text(64),
	`partial_checksum` text(64),
	`size` integer NOT NULL,
	`disk` text DEFAULT 'local' NOT NULL,
	`path` text(1024) NOT NULL,
	`mime_type` text,
	`extension` text(20),
	`original_name` text,
	`metadata` text,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer
);

CREATE UNIQUE INDEX `asset_blobs_checksum_unique` ON `asset_blobs` (`checksum`);
CREATE INDEX `asset_blobs_size_partial_checksum_idx` ON `asset_blobs` (`size`,`partial_checksum`);
CREATE INDEX `asset_blobs_mime_extension_idx` ON `asset_blobs` (`mime_type`,`extension`);
CREATE INDEX `asset_blobs_size_created_at_idx` ON `asset_blobs` (`size`,`created_at`);

CREATE TABLE `assets` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`user_id` text NOT NULL,
	`category_id` integer,
	`parent_id` integer,
	`asset_blob_id` integer,
	`type` text NOT NULL,
	`slug` text(250) NOT NULL,
	`name` text NOT NULL,
	`description` text,
	`content_created_at` text,
	`version` text(20) DEFAULT '1.0' NOT NULL,
	`is_public` integer DEFAULT false NOT NULL,
	`is_featured` integer DEFAULT false NOT NULL,
	`status` text DEFAULT 'draft' NOT NULL,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`deleted_at` integer,
	FOREIGN KEY (`user_id`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE cascade,
	FOREIGN KEY (`category_id`) REFERENCES `categories`(`id`) ON UPDATE no action ON DELETE cascade,
	FOREIGN KEY (`parent_id`) REFERENCES `assets`(`id`) ON UPDATE no action ON DELETE set null,
	FOREIGN KEY (`asset_blob_id`) REFERENCES `asset_blobs`(`id`) ON UPDATE no action ON DELETE set null
);

CREATE UNIQUE INDEX `assets_user_slug_unique` ON `assets` (`user_id`,`slug`);
CREATE INDEX `assets_parent_type_status_idx` ON `assets` (`parent_id`,`type`,`status`);
CREATE INDEX `assets_public_status_featured_idx` ON `assets` (`is_public`,`status`,`is_featured`);
CREATE INDEX `assets_user_status_idx` ON `assets` (`user_id`,`status`);
CREATE INDEX `assets_category_type_status_idx` ON `assets` (`category_id`,`type`,`status`);
CREATE INDEX `assets_created_status_idx` ON `assets` (`created_at`,`status`);

CREATE TABLE `asset_thumbnails` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`asset_id` integer NOT NULL,
	`asset_blob_id` integer NOT NULL,
	`label` text DEFAULT 'primary' NOT NULL, -- Ej: 'small', 'medium', 'large', 'animated'
	`width` integer,
	`height` integer,
	`created_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	`updated_at` integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL,
	FOREIGN KEY (`asset_id`) REFERENCES `assets`(`id`) ON UPDATE no action ON DELETE cascade,
	FOREIGN KEY (`asset_blob_id`) REFERENCES `asset_blobs`(`id`) ON UPDATE no action ON DELETE cascade
);

-- Garantiza que un asset no tenga dos miniaturas con la misma etiqueta (evita duplicados lógicos)
CREATE UNIQUE INDEX `asset_thumbnails_asset_label_unique` ON `asset_thumbnails` (`asset_id`,`label`);

-- Acelera la búsqueda de todas las miniaturas de un asset específico
CREATE INDEX `asset_thumbnails_asset_idx` ON `asset_thumbnails` (`asset_id`);
