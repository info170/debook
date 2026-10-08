// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title BookingProof
/// @notice Stores a non-transferable proof that a booking belongs to a wallet.
/// @dev Keep personal data off-chain. Use a one-way booking hash instead.
contract BookingProof {
    address public immutable ADMIN;
    uint256 public nextProofId = 1;

    mapping(address => bool) public minters;
    mapping(bytes32 => uint256) public proofIdByBooking;
    mapping(uint256 => bytes32) public bookingHashByProof;
    mapping(uint256 => address) public ownerOfProof;

    error Unauthorized();
    error ZeroAddress();
    error EmptyBookingHash();
    error BookingAlreadyProven(uint256 proofId);
    error UnknownProof();

    event MinterUpdated(address indexed account, bool enabled);
    event BookingProofCreated(
        uint256 indexed proofId,
        bytes32 indexed bookingHash,
        address indexed owner
    );

    modifier onlyAdmin() {
        _onlyAdmin();
        _;
    }

    modifier onlyMinter() {
        _onlyMinter();
        _;
    }

    function _onlyAdmin() internal view {
        if (msg.sender != ADMIN) revert Unauthorized();
    }

    function _onlyMinter() internal view {
        if (!minters[msg.sender]) revert Unauthorized();
    }

    constructor() {
        ADMIN = msg.sender;
        minters[msg.sender] = true;
        emit MinterUpdated(msg.sender, true);
    }

    /// @notice Allows the backend adapter to create proofs.
    function setMinter(address account, bool enabled) external onlyAdmin {
        if (account == address(0)) revert ZeroAddress();
        minters[account] = enabled;
        emit MinterUpdated(account, enabled);
    }

    /// @notice Creates one proof for a booking and assigns it to a wallet.
    /// @param bookingHash Hash of the off-chain booking identity.
    /// @param owner Wallet that owns the booking proof.
    function createBookingProof(bytes32 bookingHash, address owner)
        external
        onlyMinter
        returns (uint256 proofId)
    {
        if (bookingHash == bytes32(0)) revert EmptyBookingHash();
        if (owner == address(0)) revert ZeroAddress();

        uint256 existing = proofIdByBooking[bookingHash];
        if (existing != 0) revert BookingAlreadyProven(existing);

        proofId = nextProofId++;
        proofIdByBooking[bookingHash] = proofId;
        bookingHashByProof[proofId] = bookingHash;
        ownerOfProof[proofId] = owner;
        emit BookingProofCreated(proofId, bookingHash, owner);
    }

    /// @notice Checks whether a booking proof belongs to a wallet.
    function isBookingOwned(bytes32 bookingHash, address owner)
        external
        view
        returns (bool)
    {
        uint256 proofId = proofIdByBooking[bookingHash];
        return proofId != 0 && ownerOfProof[proofId] == owner;
    }

    /// @notice Returns the owner of a proof. Reverts if the proof is unknown.
    function proofOwner(uint256 proofId) external view returns (address) {
        address owner = ownerOfProof[proofId];
        if (owner == address(0)) revert UnknownProof();
        return owner;
    }
}
