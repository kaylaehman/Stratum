-- +goose Up
-- +goose StatementBegin
-- linked_vmid manually correlates a standalone Docker node to the Proxmox guest
-- (VM/LXC) it actually runs as, so its containers can nest under that guest in
-- the tree. Tri-state:
--   NULL  => AUTO   (frontend matches the guest by name)
--   0     => NONE   (force-unlinked; never nest even if names match)
--   >=100 => explicit link to that Proxmox VMID (Proxmox vmids are >=100)
ALTER TABLE nodes ADD COLUMN linked_vmid INTEGER;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE nodes DROP COLUMN linked_vmid;
-- +goose StatementEnd
