# go seof: Simple Encrypted os.File

TL/DR STATUS: It works; missing: Truncate, utilities and more testing, release, etc.

Complete implementation and drop-in replacement of golang' [`os.File`](https://golang.org/pkg/os/#File) encrypting the
underlying file with 768 bits of encryption (Triple AES256 -yes- very silly and very secure). The resulting type can be
used anywhere an [`os.File`](https://golang.org/pkg/os/#File) would be used. i.e. it can be both sequentially and
randomly read and write, at any file position for any amount of bytes.
i.e. [`Read`](https://golang.org/pkg/os/#File.Read),
[`ReadAt`](https://golang.org/pkg/os/#File.ReadAt),
[`WriteAt`](https://golang.org/pkg/os/#File.WriteAt),
[`Seek`](https://golang.org/pkg/os/#File.Seek),
[`Truncate`](https://golang.org/pkg/os/#File.Truncate), etc.

It derives a file-wide key using scrypt with a provided string password, the file is sliced into blocks of n bytes (10k
by default, decided at creation time.). Each block is encrypted and sealed using three AES256/CGM envelops, one inside
the other, achieving both [confidentiality and authenticity](https://en.wikipedia.org/wiki/Authenticated_encryption).
File wide integrity is warrantied by signing blocks and avoiding empty sparse blocks.


Performance
-----------
As a developer doing any input-output software, you want to read and write multiple bytes and not individual ones, like
the usual good practices predicates. Performance can not be expected to be as a non-encrypted file in a native
filesystem. There is no performance degradation beyond the extra work done by the encryption primitives plus the extra
ciphertext size.

Internally, `seof` holds multiple unencrypted blocks in memory, unbuffered reading and writing should not incur in any
extra encryption work, and the typical sequential reads and writes should be performant (and will not incur in
unnecessary encryption work or disk-input-output.). Because there is a limited number of blocks in memory at a time,
random reads and writes outside the current buffers, will eventually trigger encryption primitives and
disk-input-output (i.e. if a buffer content was modified, it will have to be encrypted and saved to disk, so it can be
released from memory to make space for reading another block, having to decrypt it first.)

Multiple random seek/read/write operations in a long enough file, will incur in performance penalisation as each time a
new block comes into memory from the disk, it has to be read and unencrypted, and then disposed from memory, possibly
encrypted and stored (if modified.). Encryption occurs in blocks, so changing just one byte would require encrypting and
storing a whole block (i.e. 10kb). You want to tune the quantity of in-memory blocks when opening the file; the block
size when creating it.

Sequential reads/writes with an occasional seek should be fine. This is the typical user cases that is well satisfied
with just one memory buffer, and a file block of 10kb.

File Structure
--------------

- Header: (128 bytes, 120 used)
    - uint64 Magic
    - [96]byte Script salt
    - uint32 Scrypt parameters: N, R, P.
    - uint32 Disk block size
    - [8]byte zeros (verified on open)
- A block:
    - [36]byte: nonce
    - uint32: cipherText length
    - [disk-block-size]byte: CGM stream
        - the additional data for the AEAD is an uint64 holding the block number (verified)
- Special block 0:
    - uint64: File size
    - uint32: Disk block size (must eq to the header)
    - uint32: un-encrypted block size
    - uint64: written blocks (as in number of unique nonces generated)
    - []byte: Further metadata expansion

Syncronisation
--------------

Attack vectors
--------------

- Each time a new block is written, a new nonce is generated, less than 2^32 write operations should be done in one
  particular file (and key.). Internally the implementation uses buffers and will save (and generate a new nonce) only
  when the buffer needs to be flushed to disk (i.e. file closed, explicit sync or while flushing a modified buffer.)
  if your application does a lots of random seeks and writes (constantly invalidating the blocks cache, forcing flushing
  blocks to disk, generating new nonces for the new encrypted block) you might hit that upper limit. Block 0 holds a
  counter with the number of unique nonces ever generated (which equals to the number of written and encrypted blocks).

- The weakest encryption-link is the password string used for generating the 768 bits (96 bytes) of key. A string in
  latin characters should have to be approx. 150 characters in order to hold 768 bits of entropy. You have to keep that
  in mind.

- Blocks within the same file can not be shuffled or moved to another block as the AEAD seal holds its block number as
  part of its signed plaintext. This is verified.

- Most filesystems can handle [sparse files](https://en.wikipedia.org/wiki/Sparse_file). seof supports sparse files, but
  read of never written/zeroed blocks is disabled by default to avoid a possible attack (see: XXX flag). User can create
  a new file and [`Seek`](https://golang.org/pkg/os/#File.Seek) to any part of it, write a byte, and later read it.
  Reading outside the block boundaries of the unique written byte will fail unless explicitly enabled. This is not a
  very typical user case.

  __Long explanation__: in order to keep track of blocks holding data, seof should keep a block-written-bitmap. So when
  a block is read from the disk and comes completely empty (zeroed, no AEAD seal present), but the block-written-bitmap
  accuses it was written previously, it is fair to assume the data has been lost, therefore deemed inconsistent, an IO
  error should be raised (it could have been zeroed by a malicious actor, too.). Without this block-written-bitmap, a
  zeroed block by a malicious actor and an honest empty blob in a sparse file are indistinguishable, potentially
  allowing a "selective block zero-ing attack." and failing the integrity assurances.

  If you really need this assurance, let me know, the block-written-bitmap can be done.
   